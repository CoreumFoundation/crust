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
		tagForZnet          bool
		expectedBuildParams []string
	}{
		{
			name:                "all_params",
			tagFromCommit:       "35cca0686ef057d1325ad663958e3ab069d8379d",
			tagsFromGit:         []string{"v0.0.1", "v0.0.1-rc", "tagWhichMustBeSkipped"},
			tagForZnet:          true,
			expectedBuildParams: []string{"build", "-f", "-", "/app/", "-t", "my-image:znet", "-t", "my-image:35cca06", "-t", "my-image:v0.0.1", "-t", "my-image:v0.0.1-rc"},
		},
		{
			name:                "only_znet",
			tagFromCommit:       "",
			tagsFromGit:         []string{},
			tagForZnet:          true,
			expectedBuildParams: []string{"build", "-f", "-", "/app/", "-t", "my-image:znet"},
		},
		{
			name:                "onlyFromCommit",
			tagFromCommit:       "35cca0686ef057d1325ad663958e3ab069d8379d",
			tagsFromGit:         nil,
			tagForZnet:          false,
			expectedBuildParams: []string{"build", "-f", "-", "/app/", "-t", "my-image:35cca06"},
		},
	}

	ctx := logger.WithLogger(context.Background(), logger.New(logger.Config{
		Format:  logger.FormatJSON,
		Verbose: true,
	}))

	var tags []string
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tags = getDockerBuildParams(ctx, getDockerBuildParamsInput{
				imageName:     "my-image",
				contextDir:    "/app/",
				tagFromCommit: tc.tagFromCommit, //nolint: scopelint
				tagsFromGit:   tc.tagsFromGit,   //nolint: scopelint
				tagForZnet:    tc.tagForZnet,    //nolint: scopelint
			})
			assert.Equal(t, tc.expectedBuildParams, tags) //nolint: scopelint
		})
	}
}
