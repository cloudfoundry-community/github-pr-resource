## Github PR resource

[![Go Report Card](https://goreportcard.com/badge/github.com/cloudfoundry-community/github-pr-resource)](https://goreportcard.com/report/github.com/cloudfoundry-community/github-pr-resource)
[![Docker Automated build](https://img.shields.io/docker/automated/cfcommunity/github-pr-resource.svg)](https://hub.docker.com/r/cfcommunity/github-pr-resource/)

[graphql-api]: https://developer.github.com/v4
[original-resource]: https://github.com/jtarchie/github-pullrequest-resource

A Concourse resource for pull requests on Github. Written in Go and based on the [Github V4 (GraphQL) API][graphql-api].
Inspired by [the original][original-resource], with some important differences:

- Github V4: `check` only requires 1 API call per 100th *open* pull request. (See [#costs](#costs) for more information).
- Fetch/merge: `get` will always merge a specific commit from the Pull request into the latest base.
- Metadata: `get` and `put` provides information about which commit (SHA) was used from both the PR and base.
- Webhooks: Does not implement any caching thanks to GraphQL, which means it works well with webhooks.

Make sure to check out [#migrating](#migrating) to learn more.


### Maintainance notice

This project is a fork of [telia-oss/github-pr-resource][telia_repo], which
hasn't received any maintenance for years, as telia-oss/github-pr-resource#246
can testify and explain.

As exmplained in [this comment][maintainance_takeover_comment], the project
here is to take over the maintenance, merge [pending contributions][pending_contributions]
that have been submitted as PRs to the original repo and bring significant
features, and at some point build a solution for a growing code base of
automated tests.

[telia_repo]: https://github.com/telia-oss/github-pr-resource
[maintainance_takeover_comment]: https://github.com/telia-oss/github-pr-resource/issues/246#issuecomment-2105230468
[pending_contributions]: https://github.com/telia-oss/github-pr-resource/pulls


## Source Configuration

| Parameter                   | Required | Example                          | Description                                                                                                                                                                                                        |
| --------------------------- | -------- | -------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `repository`                | Yes      | `itsdalmo/test-repository`       | The repository to target.                                                                                                                                                                                          |
| `access_token`              | Yes      |                                  | A Github Access Token with repository access, which is required for setting status on commits. (See also the note below about required permissions.)                                                               |
| `v3_endpoint`               | No       | `https://api.github.com`         | Endpoint to use for the V3 Github API (Restful).                                                                                                                                                                   |
| `v4_endpoint`               | No       | `https://api.github.com/graphql` | Endpoint to use for the V4 Github API (Graphql).                                                                                                                                                                   |
| `paths`                     | No       | `["terraform/*/*.tf"]`           | Only produce new versions if the PR includes changes to files that match one or more glob patterns or prefixes.                                                                                                    |
| `ignore_paths`              | No       | `[".ci/"]`                       | Inverse of the above. Pattern syntax is documented in [filepath.Match](https://golang.org/pkg/path/filepath/#Match), or a path prefix can be specified (e.g. `.ci/` will match everything in the `.ci` directory). |
| `disable_ci_skip`           | No       | `true`                           | Disable ability to skip builds with `[ci skip]` and `[skip ci]` in commit message or pull request title.                                                                                                           |
| `skip_ssl_verification`     | No       | `true`                           | Disable SSL/TLS certificate validation on git and API clients. Use with care!                                                                                                                                      |
| `disable_forks`             | No       | `true`                           | Disable triggering of the resource if the pull request's fork repository is different to the configured repository.                                                                                                |
| `ignore_drafts`             | No       | `false`                          | Disable triggering of the resource if the pull request is in Draft status.                                                                                                                                         |
| `required_review_approvals` | No       | `2`                              | Disable triggering of the resource if the pull request does not have at least `X` approved review(s).                                                                                                              |
| `trusted_teams`             | No       | `["wg-cf-on-k8s-bots"]`          | PRs from members of the trusted teams always trigger the resource regardless of the PR approval status.                                                                                                            |
| `trusted_users`             | No       | `["dependabot"]`                 | PRs from trusted users always trigger the resource regardless of the PR approval status.                                                                                                                           |
| `git_crypt_key`             | No       | `AEdJVENSWVBUS0VZAAAAA...`       | Base64 encoded git-crypt key. Setting this will unlock / decrypt the repository with git-crypt. To get the key simply execute `git-crypt export-key -- - | base64` in an encrypted repository.                     |
| `base_branch`               | No       | `master`                         | Name of a branch. The pipeline will only trigger on pull requests against the specified branch.                                                                                                                    |
| `labels`                    | No       | `["bug", "enhancement"]`         | The labels on the PR. The pipeline will only trigger on pull requests having at least one of the specified labels.                                                                                                 |
| `disable_git_lfs`           | No       | `true`                           | Disable Git LFS, skipping an attempt to convert pointers of files tracked into their corresponding objects when checked out into a working copy.                                                                   |
| `states`                    | No       | `["OPEN", "MERGED"]`             | The PR states to select (`OPEN`, `MERGED` or `CLOSED`). The pipeline will only trigger on pull requests matching one of the specified states. Default is ["OPEN"].                                                 |

**Notes:**

- Required permissions for `access_token`:
    - if you want github-pr-resource to work with a private repository, set
      `repo:full` permissions on the access token you create on GitHub.
    - If it is a public repository, `repo:status` is enough.
    - When using `trusted_teams`, the `read:org` scope has to be enabled.
 - If `v3_endpoint` is set, `v4_endpoint` must also be set (and the other way around).
 - Look at the [Concourse Resources documentation](https://concourse-ci.org/resources.html#resource-webhook-token)
 for webhook token configuration.
 - When using `required_review_approvals`, you may also want to enable GitHub's branch protection rules to [dismiss stale pull request approvals when new commits are pushed](https://help.github.com/en/articles/enabling-required-reviews-for-pull-requests).

## Behaviour

#### `check`

Produces new versions for all commits (after the last version) ordered by the committed date.
A version is represented as follows:

- `pr`: The pull request number.
- `commit`: The commit SHA.
- `committed`: Timestamp of when the commit was committed. Used to filter subsequent checks.
- `approved_review_count`: The number of reviews approving of the PR.

If several commits are pushed to a given PR at the same time, the last commit will be the new version.

**Note on webhooks:**
This resource does not implement any caching, so it should work well with webhooks (should be subscribed to `push` and `pull_request` events).
One thing to keep in mind however, is that pull requests that are opened from a fork and commits to said fork will not
generate notifications over the webhook. So if you have a repository with little traffic and expect pull requests from forks,
 you'll need to discover those versions with `check_every: 1m` for instance. `check` in this resource is not a costly operation,
 so normally you should not have to worry about the rate limit.

#### `get`

| Parameter            | Required | Example  | Description                                                                        |
|----------------------|----------|----------|------------------------------------------------------------------------------------|
| `skip_download`      | No       | `true`   | (deprecated) Use `no_get` on the put step instead.                                 |
| `integration_tool`   | No       | `rebase` | The integration tool to use, `merge`, `rebase` or `checkout`. Defaults to `merge`. |
| `git_depth`          | No       | `1`      | Shallow clone the repository using the `--depth` Git option                        |
| `submodules`         | No       | `true`   | Recursively clone git submodules. Defaults to false.                               |
| `list_changed_files` | No       | `true`   | Generate a list of changed files and save alongside metadata                       |
| `fetch_tags`         | No       | `true`   | Fetch tags from remote repository                                                  |

Clones the base (e.g. `master` branch) at the latest commit, and merges the pull request at the specified commit
into master. This ensures that we are both testing and setting status on the exact commit that was requested in
input. Because the base of the PR is not locked to a specific commit in versions emitted from `check`, a fresh
`get` will always use the latest commit in master and *report the SHA of said commit in the metadata*. Both the
requested version and the metadata emitted by `get` are available to your tasks as JSON:
- `.git/resource/version.json`
- `.git/resource/metadata.json`
- `.git/resource/metadata-map.json`
- `.git/resource/changed_files` (if enabled by `list_changed_files`)

The `metadata.json` file contains an array of objects, one for each key-value
pair, with a `name` key and a `value` key. In order to support the
[`load_var` step][load_var_step], another `metadata-map.json` provides the
same informtion with a plain key-value format.

[load_var_step]: https://concourse-ci.org/load-var-step.html

The information in `metadata.json` is also available as individual files in the `.git/resource` directory, e.g. the `base_sha`
is available as `.git/resource/base_sha`. For a complete list of available (individual) metadata files, please check the code
[here](https://github.com/telia-oss/github-pr-resource/blob/master/in.go#L66).

- `author`: the user login of the pull request author
- `author_email`: the e-mail address of the pull request author
- `base_name`: the base branch of the pull request
- `base_sha`: the commit of the base branch of the pull request
- `body`: the description of the pull request
- `head_name`: the branch associated with the pull request
- `head_sha`: the latest commit hash of the branch associated with the pull request
- `message`: the message of the last commit of the pull request, as designated by `head_sha`
- `pr`: the pull request ID number
- `state`: the state of the pull request, e.g. `OPEN`
- `title`: the title of the pull request
- `url`: the URL for the pull request

git-crypt encrypted repositories will automatically be decrypted when the `git_crypt_key` is set in the source configuration.

Note that, should you retrigger a build in the hopes of testing the last commit to a PR against a newer version of
the base, Concourse will reuse the volume (i.e. not trigger a new `get`) if it still exists, which can produce
unexpected results (#5). As such, re-testing a PR against a newer version of the base is best done by *pushing an
empty commit to the PR*.


#### `put`

| Parameter                  | Required | Example                              | Description                                                                                                                                                   |
|----------------------------|----------|--------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `path`                     | Yes      | `pull-request`                       | The name given to the resource in a GET step.                                                                                                                 |
| `status`                   | No       | `SUCCESS`                            | Set a status on a commit. One of `SUCCESS`, `PENDING`, `FAILURE` and `ERROR`.                                                                                 |
| `base_context`             | No       | `concourse-ci`                       | Base context (prefix) used for the status context. Defaults to `concourse-ci`.                                                                                |
| `context`                  | No       | `unit-test`                          | A context to use for the status, which is prefixed by `base_context`. Defaults to `status`.                                                                   |
| `comment`                  | No       | `hello world!`                       | A comment to add to the pull request.                                                                                                                         |
| `comment_file`             | No       | `my-output/comment.txt`              | Path to file containing a comment to add to the pull request (e.g. output of `terraform plan`).                                                               |
| `target_url`               | No       | `$ATC_EXTERNAL_URL/builds/$BUILD_ID` | The target URL for the status, where users are sent when clicking details (defaults to the Concourse build page).                                             |
| `description`              | No       | `Concourse CI build failed`          | The description status on the specified pull request.                                                                                                         |
| `description_file`         | No       | `my-output/description.txt`          | Path to file containing the description status to add to the pull request                                                                                     |
| `delete_previous_comments` | No       | `true`                               | Boolean. Previous comments made on the pull request by this resource will be deleted before making the new comment. Useful for removing outdated information. |

Note that `comment`, `comment_file` and `target_url` will all expand environment variables, so in the examples above `$ATC_EXTERNAL_URL` will be replaced by the public URL of the Concourse ATCs.
See https://concourse-ci.org/implementing-resource-types.html#resource-metadata for more details about metadata that is available via environment variables.

## Example

```yaml
resource_types:
- name: pull-request
  type: docker-image
  source:
    repository: cfcommunity/github-pr-resource

resources:
- name: pull-request
  type: pull-request
  check_every: 24h
  webhook_token: ((webhook-token))
  source:
    repository: itsdalmo/test-repository
    access_token: ((github-access-token))

jobs:
- name: test
  plan:
  - get: pull-request
    trigger: true
    version: every
  - put: pull-request
    params:
      path: pull-request
      status: pending
  - task: unit-test
    config:
      platform: linux
      image_resource:
        type: docker-image
        source: {repository: alpine/git, tag: "latest"}
      inputs:
        - name: pull-request
      run:
        path: /bin/sh
        args:
          - -xce
          - |
            cd pull-request
            git log --graph --all --color --pretty=format:"%x1b[31m%h%x09%x1b[32m%d%x1b[0m%x20%s" > log.txt
            cat log.txt
    on_failure:
      put: pull-request
      params:
        path: pull-request
        status: failure
  - put: pull-request
    params:
      path: pull-request
      status: success
```

Note: the resource image is also available as
`loggregatorbot/github-pr-resource` but with no versioning scheme and no
description. The official Docker Hub repository is
[cfcommunity/github-pr-resource](https://hub.docker.com/r/cfcommunity/github-pr-resource/).

## Costs

The Github API(s) have a rate limit of 5000 requests per hour (per user). For the V3 API this essentially
translates to 5000 requests, whereas for the V4 API (GraphQL)  the calculation is more involved:
https://developer.github.com/v4/guides/resource-limitations/#calculating-a-rate-limit-score-before-running-the-call

Ref the above, here are some examples of running `check` against large repositories and the cost of doing so:
- [concourse/concourse](https://github.com/concourse/concourse): 51 open pull requests at the time of testing. Cost 2.
- [torvalds/linux](https://github.com/torvalds/linux): 305 open pull requests. Cost 8.
- [kubernetes/kubernetes](https://github.com/kubernetes/kubernetes): 1072 open pull requests. Cost: 22.

For the other two operations the costing is a bit easier:
- `get`: Fixed cost of 1. Fetches the pull request at the given commit.
- `put`: Uses the V3 API and has a min cost of 1, +1 for each of `status`, `comment` and `comment_file` etc.

## Migrating

If you are coming from [jtarchie/github-pullrequest-resource][original-resource], its important to know that this resource is inspired by *but not a drop-in replacement for* the original. Here are some important differences:

#### New parameters:
- `source`:
  - `v4_endpoint` (see description above)
- `put`:
  - `comment` (see description above)

#### Parameters that have been renamed:
- `source`:
  - `repo` -> `repository`
  - `ci_skip` -> `disable_ci_skip` (the logic has been inverted and its `true` by default)
  - `api_endpoint` -> `v3_endpoint`
  - `base` -> `base_branch`
  - `base_url` -> `target_url`
  - `require_review_approval` -> `required_review_approvals` (`bool` to `int`)
- `get`:
  - `git.depth` -> `git_depth`
- `put`:
  - `comment` -> `comment_file` (because we added `comment`)

#### Parameters that are no longer needed:
- `src`:
  - `uri`: We fetch the URI directly from the Github API instead.
  - `private_key`: We clone over HTTPS using the access token for authentication.
  - `username`: Same as above
  - `password`: Same as above
  - `only_mergeable`: We are opinionated and simply fail to `get` if it does not merge.
- `get`:
  - `fetch_merge`: We are opinionated and always do a fetch_merge.

#### Parameters that did not make it:
- `src`:
  - `authorship_restriction`
  - `label`
  - `git_config`: You can now get the pr/author info from .git/resource/metadata.json instead
- `get`:
  - `git.*` (with the exception of `git_depth`, see above)
- `put`:
  - `merge.*`
  - `label`

#### Metadata stored in the `.git` directory

The original resource stores [a bunch of metadata][metadata] related to the
pull request as entries in `.git/config`, or plain files in the `.git/`
directory.

This resource provide all these metadata, but with possibly different names,
and only as files to be found in the `.git/resource` directory.

With this resource, no entry is added to the `.git/config` file. If you were
using the metadata stored in Git config, you need to update your code. For
example `git config --get pullrequest.url` in some Bash code can be replaced
by `echo $(< .git/resource/url)`.

Here is the list of changes:

- `.git/id` -> `.git/resource/pr`
- `.git/url` -> `.git/resource/url`
- `.git/base_branch` -> `.git/resource/base_name`
- `.git/base_sha` -> `.git/resource/base_sha`
- `.git/branch` -> `.git/resource/head_name`
- `.git/head_sha` -> `.git/resource/head_sha`
- `.git/userlogin` -> `.git/resource/author`
- `.git/body` -> `.git/resource/body`

[metadata]: https://github.com/jtarchie/github-pullrequest-resource#in-clone-the-repository-at-the-given-pull-request-ref

#### Possibly incompatible resource history

Note that if you are migrating from the original resource on a Concourse version prior to `v5.0.0`, you might
see an error `failed to unmarshal request: json: unknown field "ref"`. The solution is to rename the resource
so that the history is wiped. See telia-oss/github-pr-resource#64 for details.

