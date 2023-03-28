package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/CoreumFoundation/coreum-tools/pkg/libexec"
	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
	"github.com/CoreumFoundation/coreum-tools/pkg/must"
	"github.com/CoreumFoundation/coreum-tools/pkg/parallel"
	"github.com/CoreumFoundation/crust/exec"
)

// AppType represents the type of application.
type AppType string

// App is the interface exposed by application.
type App interface {
	// Type returns type of application
	Type() AppType

	// Info returns deployment info
	Info() DeploymentInfo

	// Name returns name of application
	Name() string

	// Deployment returns app ready to deploy
	Deployment() Deployment
}

// AppSet is the list of applications to deploy.
type AppSet []App

// Deploy deploys app in environment to the target.
func (m AppSet) Deploy(ctx context.Context, t AppTarget, config Config, spec *Spec) error {
	log := logger.Get(ctx)
	log.Info(fmt.Sprintf("Staring AppSet deployment, apps: %s", strings.Join(lo.Map(m, func(app App, _ int) string {
		return app.Name()
	}), ",")))
	err := parallel.Run(ctx, func(ctx context.Context, spawn parallel.SpawnFn) error {
		deploymentSlots := make(chan struct{}, runtime.NumCPU())
		for i := 0; i < cap(deploymentSlots); i++ {
			deploymentSlots <- struct{}{}
		}
		imagePullSlots := make(chan struct{}, 3)
		for i := 0; i < cap(imagePullSlots); i++ {
			imagePullSlots <- struct{}{}
		}

		deployments := map[string]struct {
			Deployment   Deployment
			ImageReadyCh chan struct{}
			ReadyCh      chan struct{}
		}{}
		images := map[string]chan struct{}{}
		for _, app := range m {
			deployment := app.Deployment()
			if _, exists := images[deployment.Image]; !exists {
				ch := make(chan struct{}, 1)
				ch <- struct{}{}
				images[deployment.Image] = ch
			}
			deployments[app.Name()] = struct {
				Deployment   Deployment
				ImageReadyCh chan struct{}
				ReadyCh      chan struct{}
			}{
				Deployment:   deployment,
				ImageReadyCh: images[deployment.Image],
				ReadyCh:      make(chan struct{}),
			}
		}
		for name, toDeploy := range deployments {
			if appSpec, exists := spec.Apps[name]; exists && appSpec.Info().Status == AppStatusRunning {
				close(toDeploy.ReadyCh)
				continue
			}

			appInfo := spec.Apps[name]
			toDeploy := toDeploy
			spawn("deploy."+name, parallel.Continue, func(ctx context.Context) error {
				deployment := toDeploy.Deployment

				log.Info("Deployment initialized")

				if err := ensureDockerImage(ctx, deployment.Image, imagePullSlots, toDeploy.ImageReadyCh); err != nil {
					return err
				}

				var depNames []string
				if dependencies := deployment.Requires.Dependencies; len(dependencies) > 0 {
					depNames = make([]string, 0, len(dependencies))
					for _, d := range dependencies {
						depNames = append(depNames, d.Name())
					}
					log.Info("Waiting for dependencies", zap.Strings("dependencies", depNames))
					for _, name := range depNames {
						select {
						case <-ctx.Done():
							return errors.WithStack(ctx.Err())
						case <-deployments[name].ReadyCh:
						}
					}
					log.Info("Dependencies are running now")
				}

				log.Info("Waiting for free slot for deploying the application")
				select {
				case <-ctx.Done():
					return errors.WithStack(ctx.Err())
				case <-deploymentSlots:
				}

				log.Info("Deployment started")

				info, err := deployment.Deploy(ctx, t, config)
				if err != nil {
					return err
				}
				info.DependsOn = depNames
				appInfo.SetInfo(info)

				log.Info("Deployment succeeded")

				close(toDeploy.ReadyCh)
				deploymentSlots <- struct{}{}
				return nil
			})
		}
		return nil
	})
	if err != nil {
		return err
	}
	return spec.Save()
}

// FindRunningApp returns running app of particular type and name available in app set.
func (m AppSet) FindRunningApp(appType AppType, appName string) App {
	for _, app := range m {
		if app.Type() == appType && app.Info().Status == AppStatusRunning && app.Name() == appName {
			return app
		}
	}
	return nil
}

func ensureDockerImage(ctx context.Context, image string, slots, readyCh chan struct{}) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case _, ok := <-readyCh:
		// If <-imageReadyCh blocks it means another goroutine is pulling the image
		if !ok {
			// Channel is closed, image has been already pulled, we are ready to go
			return nil
		}
	}

	log := logger.Get(ctx).With(zap.String("image", image))

	imageBuf := &bytes.Buffer{}
	imageCmd := exec.Docker("images", "-q", image)
	imageCmd.Stdout = imageBuf
	if err := libexec.Exec(ctx, imageCmd); err != nil {
		return errors.Wrapf(err, "failed to list image '%s'", image)
	}
	if imageBuf.Len() > 0 {
		log.Info("Docker image exists")
		close(readyCh)
		return nil
	}

	log.Info("Waiting for free slot for pulling the docker image")

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-slots:
	}

	log.Info("Pulling docker image")

	if err := libexec.Exec(ctx, exec.Docker("pull", image)); err != nil {
		return errors.Wrapf(err, "failed to pull docker image '%s'", image)
	}

	log.Info("Image pulled")
	close(readyCh)
	slots <- struct{}{}
	return nil
}

// DeploymentInfo contains info about deployed application.
type DeploymentInfo struct {
	// Container stores the name of the docker container where app is running - present only for apps running in docker
	Container string `json:"container,omitempty"`

	// HostFromHost is the host's hostname application binds to
	HostFromHost string `json:"hostFromHost,omitempty"`

	// HostFromContainer is the container's hostname application is listening on
	HostFromContainer string `json:"hostFromContainer,omitempty"`

	// Status indicates the status of the application
	Status AppStatus `json:"status"`

	// DependsOn is the list of other application which must be started before this one or stopped after this one
	DependsOn []string `json:"dependsOn,omitempty"`

	// Ports describe network ports provided by the application
	Ports map[string]int `json:"ports,omitempty"`
}

// Target represents target of deployment from the perspective of znet.
type Target interface {
	// Deploy deploys app set to the target
	Deploy(ctx context.Context, appSet AppSet) error

	// Stop stops apps in the app set
	Stop(ctx context.Context) error

	// Remove removes apps in the app set
	Remove(ctx context.Context) error
}

// AppTarget represents target of deployment from the perspective of application.
type AppTarget interface {
	// DeployContainer deploys container to the target
	DeployContainer(ctx context.Context, app Deployment) (DeploymentInfo, error)
}

// Prerequisites specifies list of other apps which have to be healthy before app may be started.
type Prerequisites struct {
	// Timeout tells how long we should wait for prerequisite to become healthy
	Timeout time.Duration

	// Dependencies specifies a list of health checks this app depends on
	Dependencies []HealthCheckCapable
}

// EnvVar is used to define environment variable for docker container.
type EnvVar struct {
	Name  string
	Value string
}

// Volume defines volume to be mounted inside container.
type Volume struct {
	Source      string
	Destination string
}

// Deployment represents application to be deployed.
type Deployment struct {
	// Name of the application
	Name string

	// Info stores runtime information about the app
	Info *AppInfo

	// ArgsFunc is the function returning args passed to binary
	ArgsFunc func() []string

	// Ports are the network ports exposed by the application
	Ports map[string]int

	// Requires is the list of health checks to be required before app can be deployed
	Requires Prerequisites

	// PrepareFunc is the function called before application is deployed for the first time.
	// It is a good place to prepare configuration files and other things which must or might be done before application runs.
	PrepareFunc func() error

	// ConfigureFunc is the function called after application is deployed for the first time.
	// It is a good place to connect to the application to configure it because at this stage the app's IP address is known.
	ConfigureFunc func(ctx context.Context, deployment DeploymentInfo) error

	// Image is the url of the container image
	Image string

	// EnvVarsFunc is a function defining environment variables for docker container
	EnvVarsFunc func() []EnvVar

	// Volumes defines volumes to be mounted inside the container
	Volumes []Volume

	// RunAsUser set to true causes the container to be run using uid and gid of current user.
	// It is required if container creates files inside mounted directory which is a part of app's home.
	// Otherwise, `znet` won't be able to delete them.
	RunAsUser bool

	// Entrypoint is the custom entrypoint for the container.
	Entrypoint string
}

// Deploy deploys container to the target.
func (app Deployment) Deploy(ctx context.Context, target AppTarget, config Config) (DeploymentInfo, error) {
	if err := app.preprocess(ctx, config); err != nil {
		return DeploymentInfo{}, err
	}

	info, err := target.DeployContainer(ctx, app)
	if err != nil {
		return DeploymentInfo{}, err
	}
	if err := app.postprocess(ctx, info); err != nil {
		return DeploymentInfo{}, err
	}
	return info, nil
}

func (app Deployment) preprocess(ctx context.Context, config Config) error {
	must.OK(os.MkdirAll(config.AppDir+"/"+app.Name, 0o700))

	if len(app.Requires.Dependencies) > 0 {
		waitCtx, waitCancel := context.WithTimeout(ctx, app.Requires.Timeout)
		defer waitCancel()
		if err := WaitUntilHealthy(waitCtx, app.Requires.Dependencies...); err != nil {
			return err
		}
	}

	if app.Info.Info().Status == AppStatusStopped {
		return nil
	}

	if app.PrepareFunc != nil {
		return app.PrepareFunc()
	}
	return nil
}

func (app Deployment) postprocess(ctx context.Context, info DeploymentInfo) error {
	if app.Info.Info().Status == AppStatusStopped {
		return nil
	}
	if app.ConfigureFunc != nil {
		return app.ConfigureFunc(ctx, info)
	}
	return nil
}

// NewConfigFactory creates new ConfigFactory.
func NewConfigFactory() *ConfigFactory {
	return &ConfigFactory{}
}

// ConfigFactory collects config from CLI and produces real config.
type ConfigFactory struct {
	// EnvName is the name of created environment
	EnvName string

	// Profiles defines the list of application profiles to run
	Profiles []string

	// CoredVersion defines the version of the cored to be used on start
	CoredVersion string

	// HomeDir is the path where all the files are kept
	HomeDir string

	// BinDir is the path where all binaries are present
	BinDir string

	// TestFilter is a regular expressions used to filter tests to run
	TestFilter string

	// TestGroups limits running integration tests on selected repository test group, empty means no filter
	TestGroups []string

	// VerboseLogging turns on verbose logging
	VerboseLogging bool

	// LogFormat is the format used to encode logs
	LogFormat string
}

// NewSpec returns new spec.
func NewSpec(configF *ConfigFactory) *Spec {
	specFile := configF.HomeDir + "/" + configF.EnvName + "/spec.json"
	specRaw, err := os.ReadFile(specFile)
	switch {
	case err == nil:
		spec := &Spec{
			specFile: specFile,
			configF:  configF,
		}
		must.OK(json.Unmarshal(specRaw, spec))
		return spec
	case errors.Is(err, os.ErrNotExist):
	default:
		panic(err)
	}

	spec := &Spec{
		specFile: specFile,
		configF:  configF,

		Profiles: configF.Profiles,
		Env:      configF.EnvName,
		Apps:     map[string]*AppInfo{},
	}
	return spec
}

// Spec describes running environment.
type Spec struct {
	specFile string
	configF  *ConfigFactory

	// Profiles is the list of deployed application profiles
	Profiles []string `json:"profiles"`

	// Env is the name of env
	Env string `json:"env"`

	mu sync.Mutex

	// Apps is the description of running apps
	Apps map[string]*AppInfo `json:"apps"`
}

// Verify verifies that env and profiles in config matches the ones in spec.
func (s *Spec) Verify() error {
	if s.Env != s.configF.EnvName {
		return errors.Errorf("env mismatch, spec: %s, config: %s", s.Env, s.configF.EnvName)
	}
	if !profilesCompare(s.Profiles, s.configF.Profiles) {
		return errors.Errorf("profile mismatch, spec: %s, config: %s", strings.Join(s.Profiles, ","), strings.Join(s.configF.Profiles, ","))
	}
	return nil
}

// DescribeApp adds description of running app.
func (s *Spec) DescribeApp(appType AppType, name string) *AppInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	if app, exists := s.Apps[name]; exists {
		if app.data.Type != appType {
			panic(fmt.Sprintf("app type doesn't match for application existing in spec: %s, expected: %s, got: %s", name, app.data.Type, appType))
		}
		return app
	}

	appDesc := &AppInfo{
		data: appInfoData{
			Type: appType,
		},
	}
	s.Apps[name] = appDesc
	return appDesc
}

// String converts spec to json string.
func (s *Spec) String() string {
	return string(must.Bytes(json.MarshalIndent(s, "", "  ")))
}

// Save saves spec into file.
func (s *Spec) Save() error {
	return os.WriteFile(s.specFile, []byte(s.String()), 0o600)
}

// AppStatus describes current status of an application.
type AppStatus string

const (
	// AppStatusNotDeployed ,eans that app has been never deployed.
	AppStatusNotDeployed AppStatus = ""

	// AppStatusRunning means that app is running.
	AppStatusRunning AppStatus = "running"

	// AppStatusStopped means app was running but now is stopped.
	AppStatusStopped AppStatus = "stopped"
)

type appInfoData struct {
	// Type is the type of app
	Type AppType `json:"type"`

	// Info stores app deployment information
	Info DeploymentInfo `json:"info"`
}

// AppInfo describes app running in environment.
type AppInfo struct {
	mu sync.RWMutex

	data appInfoData
}

// SetInfo sets deployment info.
func (ai *AppInfo) SetInfo(info DeploymentInfo) {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	ai.data.Info = info
}

// Info returns deployment info.
func (ai *AppInfo) Info() DeploymentInfo {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	return ai.data.Info
}

// MarshalJSON marshals data to JSON.
func (ai *AppInfo) MarshalJSON() ([]byte, error) {
	ai.mu.RLock()
	defer ai.mu.RUnlock()

	return json.Marshal(ai.data)
}

// UnmarshalJSON unmarshals data from JSON.
func (ai *AppInfo) UnmarshalJSON(data []byte) error {
	ai.mu.Lock()
	defer ai.mu.Unlock()

	return json.Unmarshal(data, &ai.data)
}

func profilesCompare(p1, p2 []string) bool {
	if len(p1) != len(p2) {
		return false
	}

	profiles := map[string]bool{}
	for _, p := range p1 {
		profiles[p] = true
	}
	for _, p := range p2 {
		if !profiles[p] {
			return false
		}
	}
	return true
}
