package resource

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v61/github"
	"github.com/shurcooL/githubv4"
	"golang.org/x/oauth2"
)

// Github for testing purposes.
//
//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_github.go . Github
type Github interface {
	ListPullRequests([]githubv4.PullRequestState) ([]*PullRequest, error)
	ListModifiedFiles(int) ([]string, error)
	ListTeamMembers(string) ([]string, error)
	PostComment(string, string) error
	GetPullRequest(string, string) (*PullRequest, error)
	GetChangedFiles(string, string) ([]ChangedFileObject, error)
	UpdateCommitStatus(string, string, string, string, string, string) error
	DeletePreviousComments(string) error
}

// GithubClient for handling requests to the Github V3 and V4 APIs.
type GithubClient struct {
	V3         *github.Client
	V4         *githubv4.Client
	Repository string
	Owner      string
}

// NewGithubClient ...
func NewGithubClient(s *Source) (*GithubClient, error) {
	owner, repository, err := parseRepository(s.Repository)
	if err != nil {
		return nil, err
	}

	// We need a transport that we can update if using GitHub App authentication
	transport := http.DefaultTransport.(*http.Transport).Clone()

	// Skip SSL verification for self-signed certificates
	// source: https://github.com/google/go-github/pull/598#issuecomment-333039238
	var ctx context.Context
	if s.SkipSSLVerification {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		insecureClient := &http.Client{Transport: transport}
		ctx = context.WithValue(context.TODO(), oauth2.HTTPClient, insecureClient)
	} else {
		ctx = context.TODO()
	}

	var client *http.Client
	if !s.UseGitHubApp {
		// Client using oauth2 wrapper
		client = oauth2.NewClient(ctx, oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: s.AccessToken},
		))
	} else {
		var ghAppInstallationTransport *ghinstallation.Transport
		ghAppInstallationTransport, err = ghinstallation.New(transport, s.ApplicationID, s.InstallationID, []byte(s.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("failed to generate application installation access token using private key: %s", err)
		}

		// Client using ghinstallation transport
		client = &http.Client{
			Transport: ghAppInstallationTransport,
		}
	}

	var v3 *github.Client
	if s.V3Endpoint != "" {
		endpoint, err := url.Parse(s.V3Endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to parse v3 endpoint: %s", err)
		}
		v3, err = github.NewEnterpriseClient(endpoint.String(), endpoint.String(), client)
		if err != nil {
			return nil, err
		}
	} else {
		v3 = github.NewClient(client)
	}

	var v4 *githubv4.Client
	if s.V4Endpoint != "" {
		endpoint, err := url.Parse(s.V4Endpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to parse v4 endpoint: %s", err)
		}
		v4 = githubv4.NewEnterpriseClient(endpoint.String(), client)
		if err != nil {
			return nil, err
		}
	} else {
		v4 = githubv4.NewClient(client)
	}

	return &GithubClient{
		V3:         v3,
		V4:         v4,
		Owner:      owner,
		Repository: repository,
	}, nil
}

// ListPullRequests gets the last commit on all pull requests with the matching state.
func (m *GithubClient) ListPullRequests(prStates []githubv4.PullRequestState) ([]*PullRequest, error) {
	var query struct {
		Repository struct {
			PullRequests struct {
				Edges []struct {
					Node struct {
						PullRequestObject
						Reviews struct {
							Nodes []struct {
								AuthorCanPushToRepository bool
							}
						} `graphql:"reviews(last: $prReviewsLast,states: $prReviewStates)"`
						Commits struct {
							Edges []struct {
								Node struct {
									Commit CommitObject
								}
							}
						} `graphql:"commits(last:$commitsLast)"`
						Labels struct {
							Edges []struct {
								Node struct {
									LabelObject
								}
							}
						} `graphql:"labels(first:$labelsFirst)"`
					}
				}
				PageInfo struct {
					EndCursor   githubv4.String
					HasNextPage bool
				}
			} `graphql:"pullRequests(first:$prFirst,states:$prStates,after:$prCursor)"`
		} `graphql:"repository(owner:$repositoryOwner,name:$repositoryName)"`
	}

	vars := map[string]interface{}{
		"repositoryOwner": githubv4.String(m.Owner),
		"repositoryName":  githubv4.String(m.Repository),
		"prFirst":         githubv4.Int(100),
		"prReviewsLast":   githubv4.Int(100),
		"prStates":        prStates,
		"prCursor":        (*githubv4.String)(nil),
		"commitsLast":     githubv4.Int(1),
		"prReviewStates":  []githubv4.PullRequestReviewState{githubv4.PullRequestReviewStateApproved},
		"labelsFirst":     githubv4.Int(100),
	}

	var response []*PullRequest
	for {
		if err := m.V4.Query(context.TODO(), &query, vars); err != nil {
			return nil, err
		}
		for _, p := range query.Repository.PullRequests.Edges {
			labels := make([]LabelObject, len(p.Node.Labels.Edges))
			numApprovals := 0
			for _, review := range p.Node.Reviews.Nodes {
				if review.AuthorCanPushToRepository {
					numApprovals++
				}
			}
			for _, l := range p.Node.Labels.Edges {
				labels = append(labels, l.Node.LabelObject)
			}

			for _, c := range p.Node.Commits.Edges {
				response = append(response, &PullRequest{
					PullRequestObject:   p.Node.PullRequestObject,
					Tip:                 c.Node.Commit,
					ApprovedReviewCount: numApprovals,
					Labels:              labels,
				})
			}
		}
		if !query.Repository.PullRequests.PageInfo.HasNextPage {
			break
		}
		vars["prCursor"] = query.Repository.PullRequests.PageInfo.EndCursor
	}
	return response, nil
}

func (m *GithubClient) ListTeamMembers(team string) ([]string, error) {
	var query struct {
		Organization struct {
			Team struct {
				Members struct {
					Nodes []struct {
						Login string
					}
				}
			} `graphql:"team(slug:$teamName)"`
		} `graphql:"organization(login:$orgName)"`
	}

	vars := map[string]interface{}{
		"teamName": githubv4.String(team),
		"orgName":  githubv4.String(m.Owner),
	}

	teamMembers := []string{}
	if err := m.V4.Query(context.Background(), &query, vars); err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	for _, user := range query.Organization.Team.Members.Nodes {
		teamMembers = append(teamMembers, user.Login)
	}
	return teamMembers, nil
}

// ListModifiedFiles in a pull request (not supported by V4 API).
func (m *GithubClient) ListModifiedFiles(prNumber int) ([]string, error) {
	var files []string

	opt := &github.ListOptions{
		PerPage: 100,
	}
	for {
		result, response, err := m.V3.PullRequests.ListFiles(
			context.TODO(),
			m.Owner,
			m.Repository,
			prNumber,
			opt,
		)
		if err != nil {
			return nil, err
		}
		for _, f := range result {
			files = append(files, *f.Filename)
		}
		if response.NextPage == 0 {
			break
		}
		opt.Page = response.NextPage
	}
	return files, nil
}

// PostComment to a pull request or issue.
func (m *GithubClient) PostComment(prNumber, comment string) error {
	pr, err := strconv.Atoi(prNumber)
	if err != nil {
		return fmt.Errorf("failed to convert pull request number to int: %s", err)
	}

	_, _, err = m.V3.Issues.CreateComment(
		context.TODO(),
		m.Owner,
		m.Repository,
		pr,
		&github.IssueComment{
			Body: github.String(comment),
		},
	)
	return err
}

// GetChangedFiles ...
func (m *GithubClient) GetChangedFiles(prNumber string, commitRef string) ([]ChangedFileObject, error) {
	pr, err := strconv.Atoi(prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to convert pull request number to int: %s", err)
	}

	var cfo []ChangedFileObject

	var filequery struct {
		Repository struct {
			PullRequest struct {
				Files struct {
					Edges []struct {
						Node struct {
							ChangedFileObject
						}
					} `graphql:"edges"`
					PageInfo struct {
						EndCursor   githubv4.String
						HasNextPage bool
					} `graphql:"pageInfo"`
				} `graphql:"files(first:$changedFilesFirst, after: $changedFilesEndCursor)"`
			} `graphql:"pullRequest(number:$prNumber)"`
		} `graphql:"repository(owner:$repositoryOwner,name:$repositoryName)"`
	}

	offset := ""

	for {
		vars := map[string]interface{}{
			"repositoryOwner":       githubv4.String(m.Owner),
			"repositoryName":        githubv4.String(m.Repository),
			"prNumber":              githubv4.Int(pr),
			"changedFilesFirst":     githubv4.Int(100),
			"changedFilesEndCursor": githubv4.String(offset),
		}

		if err := m.V4.Query(context.TODO(), &filequery, vars); err != nil {
			return nil, err
		}

		for _, f := range filequery.Repository.PullRequest.Files.Edges {
			cfo = append(cfo, ChangedFileObject{Path: f.Node.Path})
		}

		if !filequery.Repository.PullRequest.Files.PageInfo.HasNextPage {
			break
		}

		offset = string(filequery.Repository.PullRequest.Files.PageInfo.EndCursor)
	}

	return cfo, nil
}

// GetPullRequest ...
func (m *GithubClient) GetPullRequest(prNumber, commitRef string) (*PullRequest, error) {
	pr, err := strconv.Atoi(prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to convert pull request number to int: %s", err)
	}

	var query struct {
		Repository struct {
			PullRequest struct {
				PullRequestObject
				Commits struct {
					Edges []struct {
						Node struct {
							Commit CommitObject
						}
					}
				} `graphql:"commits(last:$commitsLast)"`
			} `graphql:"pullRequest(number:$prNumber)"`
		} `graphql:"repository(owner:$repositoryOwner,name:$repositoryName)"`
	}

	vars := map[string]interface{}{
		"repositoryOwner": githubv4.String(m.Owner),
		"repositoryName":  githubv4.String(m.Repository),
		"prNumber":        githubv4.Int(pr),
		"commitsLast":     githubv4.Int(100),
	}

	// TODO: Pagination - in case someone pushes > 100 commits before the build has time to start :p
	if err := m.V4.Query(context.TODO(), &query, vars); err != nil {
		return nil, err
	}

	for _, c := range query.Repository.PullRequest.Commits.Edges {
		if c.Node.Commit.OID == commitRef {
			// Return as soon as we find the correct ref.
			return &PullRequest{
				PullRequestObject: query.Repository.PullRequest.PullRequestObject,
				Tip:               c.Node.Commit,
			}, nil
		}
	}

	// Return an error if the commit was not found
	return nil, fmt.Errorf("commit with ref '%s' does not exist", commitRef)
}

// UpdateCommitStatus for a given commit (not supported by V4 API).
func (m *GithubClient) UpdateCommitStatus(commitRef, baseContext, statusContext, status, targetURL, description string) error {
	if baseContext == "" {
		baseContext = "concourse-ci"
	}

	if statusContext == "" {
		statusContext = "status"
	}

	if targetURL == "" {
		targetURL = strings.Join([]string{os.Getenv("ATC_EXTERNAL_URL"), "builds", os.Getenv("BUILD_ID")}, "/")
	}

	if description == "" {
		description = fmt.Sprintf("Concourse CI build %s", status)
	}

	_, _, err := m.V3.Repositories.CreateStatus(
		context.TODO(),
		m.Owner,
		m.Repository,
		commitRef,
		&github.RepoStatus{
			State:       github.String(strings.ToLower(status)),
			TargetURL:   github.String(targetURL),
			Description: github.String(description),
			Context:     github.String(path.Join(baseContext, statusContext)),
		},
	)
	return err
}

func (m *GithubClient) DeletePreviousComments(prNumber string) error {
	pr, err := strconv.Atoi(prNumber)
	if err != nil {
		return fmt.Errorf("failed to convert pull request number to int: %s", err)
	}

	var getComments struct {
		Viewer struct {
			Login string
		}
		Repository struct {
			PullRequest struct {
				Id       string
				Comments struct {
					Edges []struct {
						Node struct {
							DatabaseId int64
							Author     struct {
								Login string
							}
						}
					}
				} `graphql:"comments(last:$commentsLast)"`
			} `graphql:"pullRequest(number:$prNumber)"`
		} `graphql:"repository(owner:$repositoryOwner,name:$repositoryName)"`
	}

	vars := map[string]interface{}{
		"repositoryOwner": githubv4.String(m.Owner),
		"repositoryName":  githubv4.String(m.Repository),
		"prNumber":        githubv4.Int(pr),
		"commentsLast":    githubv4.Int(100),
	}

	if err := m.V4.Query(context.TODO(), &getComments, vars); err != nil {
		return err
	}

	for _, e := range getComments.Repository.PullRequest.Comments.Edges {
		if cleanBotUserName(e.Node.Author.Login) == cleanBotUserName(getComments.Viewer.Login) {
			_, err := m.V3.Issues.DeleteComment(context.TODO(), m.Owner, m.Repository, e.Node.DatabaseId)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func parseRepository(s string) (string, string, error) {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		return "", "", errors.New("malformed repository")
	}
	return parts[0], parts[1], nil
}

func cleanBotUserName(username string) string {
	if strings.HasSuffix(username, "[bot]") {
		return strings.TrimSuffix(username, "[bot]")
	}
	return username
}
