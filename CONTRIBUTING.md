First off, thank you for considering contributing to *vertica-kubernetes* and helping make it even better than it is today!

This document will guide you through the contribution process. There are a number of ways you can help:

 - [Bug Reports](#bug-reports)
 - [Feature Requests](#feature-requests)
 - [Code Contributions](#code-contributions)
 
# Bug Reports

If you find a bug, submit an [issue](https://github.com/vertica/vertica-kubernetes/issues) with a complete and reproducible bug report. If the issue can't be reproduced, it will be closed. If you opened an issue, but figured out the answer later on your own, comment on the issue to let people know, then close the issue.

For issues (e.g. security related issues) that are **not suitable** to be reported publicly on the GitHub issue system, report your issues to [Vertica open source team](mailto:vertica-opensrc@microfocus.com) directly or file a case with Vertica support if you have a support account.

# Feature Requests

Feel free to share your ideas for how to improve *vertica-kubernetes*. We’re always open to suggestions.
You can open an [issue](https://github.com/vertica/vertica-kubernetes/issues)
with details describing what feature(s) you'd like added or changed.

# Code Contributions

## Step 1: Fork

Fork the project [on Github](https://github.com/vertica/vertica-kubernetes) and check out your copy locally.

```shell
git clone git@github.com:vertica/vertica-kubernetes.git
cd vertica-kubernetes
```

Your GitHub repository will be called "origin" in Git. You should also setup **vertica/vertica-kubernetes** as an "upstream" remote.

```shell
git remote add upstream git@github.com:vertica/vertica-kubernetes.git
git fetch upstream
```

### Configure Git for the first time

Make sure git knows your [name](https://help.github.com/articles/setting-your-username-in-git/ "Set commit username in Git") and [email address](https://help.github.com/articles/setting-your-commit-email-address-in-git/ "Set commit email address in Git"):

```shell
git config --global user.name "John Smith"
git config --global user.email "email@example.com"
```

## Step 2: Branch

Create a new branch for the work with a descriptive name:

```shell
git checkout -b my-fix-branch
```

## Step 3: Setup the development environment

Refer to the [develolper](DEVELOPER.md) document on how to setup your environment.

## Step 4: Implement your fix or feature

At this point, you're ready to make your changes.

### License Headers

Every file in this project must use the following Apache 2.0 header (with the appropriate year or years in the "[yyyy]" box; if a copyright statement from another party is already present in the code, you may add the statement on top of the existing copyright statement):

```
Copyright (c) [yyyy] Micro Focus or one of its affiliates.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

### Commits

Make some changes on your branch, then stage and commit as often as necessary:

```shell
git add .
git commit -m 'Add new e2e test for nodePort'
```

When writing the commit message, try to describe precisely what the commit does. The commit message should be in lines of 72 chars maximum. Include the issue number `#N`, if the commit is related to an issue.

### Tests

Each code change should have a corresponding test that covers it.  We have two levels of test: unit tests and end-to-end (e2e) tests.  It is desirable to add both types of tests.  The e2e tests can take a while to run, so often times adding to an existing test is sufficient.

## Step 6: Push and Rebase

You can publish your work on GitHub just by doing:

```shell
git push origin my-fix-branch
```

When you go to your GitHub page, you will notice commits made on your local branch is pushed to the remote repository.

When upstream (vertica/vertica-kubernetes) has changed, you should rebase your work. The **rebase** command creates a linear history by moving your local commits onto the tip of the upstream commits.

You can rebase your branch locally and force-push to your GitHub repository by doing:

```shell
git checkout my-fix-branch
git fetch upstream
git rebase upstream/master
git push -f origin my-fix-branch
```


## Step 7: Make a Pull Request

When you think your work is ready to be pulled into *vertica-kubernetes*, you should create a pull request(PR) at GitHub.

A good pull request means:
 - commits with one logical change in each
 - well-formed messages for each commit
 - documentation and tests, if needed

Go to your fork in GitHub and [make a Pull Request](https://help.github.com/articles/creating-a-pull-request/) to `vertica:main`. 

### Sign the CLA
Before we can accept a pull request, we first ask people to sign a Contributor License Agreement (or CLA). We ask this so that we know that contributors have the right to donate the code. You should notice a comment from **CLAassistant** on your pull request page, follow this comment to sign the CLA electronically. 

### Review
Pull requests are usually reviewed within a few days. If there are comments to address, apply your changes in new commits, rebase your branch and force-push to the same branch, re-run the test suite to ensure tests are still passing. 

In order to produce a clean commit history, our maintainers would do squash merging once your PR is approved, which means combining all commits of your PR into a single commit in the master branch.

After your pull request is merged, you can safely delete your branch and pull the changes from the upstream repository.

