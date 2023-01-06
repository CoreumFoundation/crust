package docker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
)

func TestGetTagsForDockerImage(t *testing.T) {
	testCases := []struct {
		name                string
		tagFromCommit       string
		tagsFromGit         []string
		expectedBuildParams []string
	}{
		{
			name:                "all_params",
			tagFromCommit:       "35cca0686ef057d1325ad663958e3ab069d8379d",
			tagsFromGit:         []string{"v0.0.1", "v0.0.1-rc"},
			expectedBuildParams: []string{"build", "-t", "my-image:znet", "-t", "my-image:35cca06", "-t", "my-image:v0.0.1", "-t", "my-image:v0.0.1-rc", "-f", "-", "/app/"},
		},
		{
			name:                "onlyFromCommitAndZnet",
			tagFromCommit:       "35cca0686ef057d1325ad663958e3ab069d8379d",
			tagsFromGit:         []string{"allGitTagsMustBeSkipped", "v0.0.1-", "0.0.1", "v0.0.1-ra", "v0.0.1rc", "v0.0.1.rc"},
			expectedBuildParams: []string{"build", "-t", "my-image:znet", "-t", "my-image:35cca06", "-f", "-", "/app/"},
		},
	}

	ctx := logger.WithLogger(context.Background(), logger.New(logger.Config{
		Format:  logger.FormatJSON,
		Verbose: true,
	}))

	var tags []string
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tags = getDockerBuildParams(ctx, dockerBuildParamsInput{
				imageName:  "my-image",
				contextDir: "/app/",
				commitHash: tc.tagFromCommit, //nolint: scopelint
				tags:       tc.tagsFromGit,   //nolint: scopelint
			})
			assert.Equal(t, tc.expectedBuildParams, tags) //nolint: scopelint
		})
	}
}
