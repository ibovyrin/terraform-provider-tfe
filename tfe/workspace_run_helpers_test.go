package tfe

import (
	"testing"

	"github.com/golang/mock/gomock"
	tfe "github.com/hashicorp/go-tfe"
	tfemocks "github.com/hashicorp/go-tfe/mocks"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func MockRunsListForWorkspaceQueue(t *testing.T, client *tfe.Client, workspaceIDWithExpectedRun string, workspaceIDWithUnexpectedRun string) {
	ctrl := gomock.NewController(t)
	mockRunsAPI := tfemocks.NewMockRuns(ctrl)

	runListWithExpectedIDNotIncluded := tfe.RunList{
		Items: []*tfe.Run{
			{
				ID:     "run-01",
				Status: tfe.RunPending,
			},
		},
		Pagination: &tfe.Pagination{
			CurrentPage: 1,
			TotalPages:  1,
			TotalCount:  1,
		},
	}

	runListWithExpectedIDIncluded := tfe.RunList{
		Items: []*tfe.Run{
			{
				ID:     "run-01",
				Status: tfe.RunPending,
			},
			{
				ID:     "run-02",
				Status: tfe.RunPending,
			},
			{
				ID:     "run-03",
				Status: tfe.RunApplied,
			},
			{
				ID:     "run-04",
				Status: tfe.RunPending,
			},
			{
				ID:     "run-05",
				Status: tfe.RunApplying,
			},
		},
		Pagination: &tfe.Pagination{
			CurrentPage: 1,
			TotalPages:  1,
			TotalCount:  4,
		},
	}

	mockRunsAPI.
		EXPECT().
		List(gomock.Any(), workspaceIDWithExpectedRun, gomock.Any()).
		Return(&runListWithExpectedIDIncluded, nil).
		AnyTimes()

	mockRunsAPI.
		EXPECT().
		List(gomock.Any(), workspaceIDWithUnexpectedRun, gomock.Any()).
		Return(&runListWithExpectedIDNotIncluded, nil).
		AnyTimes()

	mockRunsAPI.
		EXPECT().
		List(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, tfe.ErrInvalidOrg).
		AnyTimes()

	client.Runs = mockRunsAPI
}

func TestReadRunPositionInWorkspaceQueue(t *testing.T) {
	defaultOrganization := "my-org"
	client := testTfeClient(t, testClientOptions{defaultOrganization: defaultOrganization})
	MockRunsListForWorkspaceQueue(t, client, "ws-1", "ws-2")

	testCases := map[string]struct {
		currentRunID string
		workspace    string
		err          bool
		returnVal    int
	}{
		"when fetching run list returns error": {
			"run-02",
			"ws-unknown",
			true,
			0,
		},
		"when runID is found in the workspace queue": {
			"run-02",
			"ws-1",
			false,
			2,
		},
		"when runID is not found in the workspace queue": {
			"run-02",
			"ws-2",
			false,
			0,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			position, err := readRunPositionInWorkspaceQueue(
				ConfiguredClient{Client: client, Organization: defaultOrganization},
				testCase.currentRunID,
				testCase.workspace,
				false,
				&tfe.Run{
					ID:     "run-02",
					Status: tfe.RunApplying,
				})

			if (err != nil) != testCase.err {
				t.Fatalf("expected error is %t, got %v", testCase.err, err)
			}

			if position != testCase.returnVal {
				t.Fatalf("expected returned value is %d, got %v", testCase.returnVal, position)
			}
		})
	}
}

func MockRunsQueueForOrg(t *testing.T, client *tfe.Client, orgName string, orgNameWithRun string) {
	ctrl := gomock.NewController(t)
	mockRunQueueAPI := tfemocks.NewMockOrganizations(ctrl)

	runQueueWithExpectedIDNotIncluded := tfe.RunQueue{
		Items: []*tfe.Run{
			{
				ID:              "run-01",
				Status:          tfe.RunPending,
				PositionInQueue: 0,
			},
		},
		Pagination: &tfe.Pagination{
			CurrentPage: 1,
			TotalPages:  1,
			TotalCount:  1,
		},
	}

	runQueueWithExpectedIDIncluded := tfe.RunQueue{
		Items: []*tfe.Run{
			{
				ID:              "run-01",
				Status:          tfe.RunPending,
				PositionInQueue: 0,
			},
			{
				ID:              "run-02",
				Status:          tfe.RunPending,
				PositionInQueue: 1,
			},
		},
		Pagination: &tfe.Pagination{
			CurrentPage: 1,
			TotalPages:  1,
			TotalCount:  2,
		},
	}

	mockRunQueueAPI.
		EXPECT().
		ReadRunQueue(gomock.Any(), orgNameWithRun, gomock.Any()).
		Return(&runQueueWithExpectedIDIncluded, nil).
		AnyTimes()

	mockRunQueueAPI.
		EXPECT().
		ReadRunQueue(gomock.Any(), orgName, gomock.Any()).
		Return(&runQueueWithExpectedIDNotIncluded, nil).
		AnyTimes()

	mockRunQueueAPI.
		EXPECT().
		ReadRunQueue(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, tfe.ErrInvalidOrg).
		AnyTimes()

	client.Organizations = mockRunQueueAPI
}

func TestReadRunPositionInOrgQueue(t *testing.T) {
	defaultOrganization := "my-org"
	client := testTfeClient(t, testClientOptions{defaultOrganization: defaultOrganization})
	MockRunsQueueForOrg(t, client, "another-org", defaultOrganization)

	testCases := map[string]struct {
		orgName   string
		err       bool
		returnVal int
	}{
		"when fetching organization run queue returns error": {
			"unknown-org",
			true,
			0,
		},
		"when run is found in organization run queue": {
			defaultOrganization,
			false,
			1,
		},
		"when run is not found in organization run queue": {
			"another-org",
			false,
			0,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			position, err := readRunPositionInOrgQueue(
				ConfiguredClient{Client: client, Organization: defaultOrganization},
				"run-02",
				testCase.orgName,
			)

			if (err != nil) != testCase.err {
				t.Fatalf("expected error is %t, got %v", testCase.err, err)
			}

			if position != testCase.returnVal {
				t.Fatalf("expected returned value is %d, got %v", testCase.returnVal, position)
			}
		})
	}
}

func TestCreateWorkspaceRun(t *testing.T) {
	testCases := map[string]struct {
		isDestroyRun bool
	}{
		"when destroy block is not set OnDestroy": {
			true,
		},
		"when apply block is not set OnCreate": {
			false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			d := &schema.ResourceData{}
			client := testTfeClient(t, testClientOptions{defaultOrganization: "my-org"})
			meta := ConfiguredClient{Client: client, Organization: "my-org"}
			currentRetryAttempts := 0

			err := createWorkspaceRun(d, meta, testCase.isDestroyRun, currentRetryAttempts)

			if err != nil {
				t.Fatalf("expected error is nil, got %v", err)
			}
		})
	}
}
