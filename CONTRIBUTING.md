# Contributing to OpenEBS jiva

OpenEBS uses the standard GitHub pull requests process to review and accept contributions.  There are several areas that could use your help. For starters, you could help in improving the sections in this document by either creating a new issue describing the improvement or submitting a pull request to this repository. 

* If you are a first-time contributor, please see [Steps to Contribute](#steps-to-contribute).
* If you have documentation improvement ideas, go ahead and create a pull request. See [Pull Request checklist](#pull-request-checklist).
* If you would like to make code contributions, please start with [Setting up the Development Environment](./developer-setup.md).
* If you would like to work on something more involved, please connect with the OpenEBS Contributors. See [OpenEBS Community](https://github.com/openebs/openebs/tree/HEAD/community).

## Steps to Contribute

OpenEBS is an Apache 2.0 Licensed project and all your commits should be signed with Developer Certificate of Origin. See [Sign your work](#sign-your-work).

* Find an issue to work on or create a new issue. 
  - The issues are maintained at [issues](https://github.com/openebs/jiva/issues). 
  - You can pick up from a list of [good-first-issues](https://github.com/openebs/jiva/labels/good%20first%20issue).
  - For more ideas on what to work checkout [ROADMAP.md](https://github.com/openebs/openebs/blob/main/ROADMAP.md).
* Fork this repository and raise a pull request with your proposed changes. See [Pull Request checklist](#pull-request-checklist).

## Pull Request Checklist

* Fork the repository on GitHub.
* Create a branch from where you want to base your work (usually develop).
* Make your changes. If you are working on code contributions, please see [Setting up the Development Environment](./developer-setup.md).
* Rebase to the current develop branch before submitting your pull request.
* Commits should be as small as possible. Each commit should follow the checklist below:
  - For code changes, add tests relevant to the fixed bug or new feature.
  - Pass the compile and tests - includes spell checks, formatting, etc.
  - Commit header (first line) should convey what changed.
  - Commit body should include details such as why the changes are required and how the proposed changes.
  - [DCO Signed](#sign-your-work).
* Push your changes to the branch in your fork of the repository.
* Raise a new [Pull Request](https://github.com/openebs/jiva/pulls).
* If your PR is not getting reviewed or you need a specific person to review it, please reach out to the OpenEBS Contributors. See [OpenEBS Community](https://github.com/openebs/openebs/tree/HEAD/community).

---

## Sign your work

We use the Developer Certificate of Origin (DCO) as an additional safeguard for the OpenEBS project. This is a well established and widely used mechanism to assure that contributors have confirmed their right to license their contribution under the project's license. Please read [dcofile](https://github.com/openebs/openebs/blob/HEAD/contribute/developer-certificate-of-origin). If you can certify it, then just add a line to every git commit message:

Please certify it by just adding a line to every git commit message. Any PR with Commits which does not have DCO Signoff will not be accepted:

````
  Signed-off-by: Random J Developer <random@developer.example.org>
````

Use your real name (sorry, no pseudonyms or anonymous contributions). The email id should match the email id provided in your GitHub profile.
If you set your `user.name` and `user.email` in git config, you can sign your commit automatically with `git commit -s`.

You can also use git [aliases](https://git-scm.com/book/tr/v2/Git-Basics-Git-Aliases) like `git config --global alias.ci 'commit -s'`. Now you can commit with `git ci` and the commit will be signed.

---
### Make your changes and build them

