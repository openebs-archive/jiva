# Release Process

**jiva** follows the [OpenEBS release](https://github.com/openebs/openebs/blob/HEAD/RELEASE.md) process for major and minor releases. The scope of the release is determined by contributor availability. The scope is published in the [Release Tracker Projects](https://github.com/orgs/openebs/projects). The release process involves the following steps:

- After all the scoped features are committed to `develop` branch, a release branch is created. Name of the release branch should follow the naming convention of `v<major-release>.<minor-release>.x`. Example: v1.9.x.
- Create release candidate (`RC`) tags from the release branch and run release pipelines.
- Create release tag once the verification is complete.
- Update the [changelog](./changelogs/released/).
- Update and release a new version of [Jiva Operator](https://github.com/openebs/jiva-operator/blob/HEAD/RELEASE.md).

Patch releases are created on demand, depending on the severity of the fixes. The fixes will have to merged into the `develop` branch and cherry picked into the corresponding release branches and a new patch release is created.

## Released Containers Images

Multi-arch containers are pushed to different container registeries, via the [Github Actions release workflow](./.github/workflows/release.yaml).

The following container images are generated from this repo:
- Docker Hub (Default)
  - openebs/jiva
- Quay.io
  - quay.io/jiva
- GHCR
  - ghcr.io/jiva

## Update CHANGELOG

Once a release tag is created, raise a changelog PR to `develop` branch that updates the following:
- Move the fixes that went into the release from `changeslogs/unreleased` to `changeslogs/released/<release tag>`. 
- Update [CHANGELOG.md] with fixes that went into the current release.
