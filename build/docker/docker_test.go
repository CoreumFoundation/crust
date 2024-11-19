package docker

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/CoreumFoundation/coreum-tools/pkg/logger"
)

func TestGetTagsForDockerImage(t *testing.T) {
	testCases := []struct {
		name              string
		versionFromCommit string
		versionsFromGit   []string
		expectedBuildTags []string
	}{
		{
			name:              "all_params",
			versionFromCommit: "35cca0686ef057d1325ad663958e3ab069d8379d",
			versionsFromGit:   []string{"v0.0.1", "v0.0.1-rc"},
			expectedBuildTags: []string{
				"user/my-image:other",
				"user/my-image:35cca06",
				"user/my-image:v0.0.1",
				"user/my-image:v0.0.1-rc",
			},
		},
		{
			name:              "onlyFromCommitAndOther",
			versionFromCommit: "35cca0686ef057d1325ad663958e3ab069d8379d",
			versionsFromGit:   []string{"allGitTagsMustBeSkipped", "v0.0.1-", "0.0.1", "v0.0.1-ra", "v0.0.1rc", "v0.0.1.rc"},
			expectedBuildTags: []string{"user/my-image:other", "user/my-image:35cca06"},
		},
	}

	ctx := logger.WithLogger(context.Background(), logger.New(logger.Config{
		Format:  logger.FormatJSON,
		Verbose: true,
	}))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := getDockerBuildParams(ctx, dockerBuildParamsInput{
				imageName:     "my-image",
				contextDir:    "/app/",
				commitHash:    tc.versionFromCommit,
				gitVersions:   tc.versionsFromGit,
				otherVersions: []string{"other"},
				username:      "user",
			})
			for i, arg := range args {
				if arg == "-t" {
					assert.Contains(t, tc.expectedBuildTags, args[i+1])
				}
			}
		})
	}
}
