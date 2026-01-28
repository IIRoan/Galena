## North Star

You’re building an “OCI-native OS appliance” that *happens* to be a desktop (or host): the whole base system ships as a signed bootable container image, updates are atomic, and rollbacks are a boot menu decision or a single command. That is exactly the bootc model (“transactional, in-place operating system updates using OCI/Docker container images”). ([docs.fedoraproject.org][1])

Bluefin/Universal Blue already solved most of the “sharp edges” (build structure, digest pinning, variant matrix, ISO/disk images, ujust UX). Your plan of attack is: **clone the manufacturing pattern**, start tiny, then gradually replace the orchestration layer with Go without ripping out the stable core.

---

# 1) High-level architecture choice

## Base image strategy

### Recommended for your first year: Universal Blue “Silverblue-like” base

Start from **`ghcr.io/ublue-os/silverblue-main`** (or a sibling base for your desktop choice) because:

* Bluefin itself builds on **`silverblue-main`** as the GNOME atomic desktop foundation. ([DeepWiki][2])
* Universal Blue base images are explicitly positioned as the default/recommended bases for custom images in the ecosystem. ([BlueBuild][3])
* You immediately inherit the community’s bootc-ready plumbing and a desktop that’s already “atomic-first”.

This gets you to a booting desktop fastest, and it matches your “Chromebook-like reliability, developer-oriented first” constraint. Universal Blue literally frames its mission as “reliability of a Chromebook… with the flexibility… of a traditional Linux desktop.” ([universal-blue.org][4])

### Later (optional): “LTS-ish” base on CentOS bootc or other bootc bases

Once you’re comfortable, you can support a second track (or a separate product) based on the Fedora/CentOS bootc reference images (or even Ubuntu bootc variants). Bluefin’s own docs/blogs highlight that their OCI layout lets you assemble a “Bluefin-like” system by consuming shared OCI layers, and even mentions an Ubuntu-bootc experiment. ([docs.projectbluefin.io][5])

**Translation:** don’t lock yourself into “Fedora forever” in your repo structure. Lock yourself into **bootc + OCI + signed artifacts**.

---

## What stays immutable vs mutable (and why)

### Immutable (image-controlled)

* **`/usr`**: everything you want Chromebook-like reliability for (kernel, systemd, drivers, base userland) lives here, shipped in the image. This is why Bluefin precompiles kernel modules during the build: users can’t compile into an immutable `/usr` at runtime. ([DeepWiki][6])
* **Most defaults in `/usr/etc`**: ship sane defaults here so users can override in `/etc` without “owning” the base image. Bluefin documents this override model. ([docs.projectbluefin.io][7])

### Mutable (state and user data)

* **`/var`**: persistent state. Bootc rollbacks reorder deployments; user content in `/var` is not affected. ([GitLab][8])
* **`/etc`**: mutable, but with an important bootc quirk: rolling back does **not** carry forward `/etc` changes; it reverts `/etc` to the older deployment’s state. Bootc docs explicitly advise copying important modified configs into `/var` if you need them across a rollback. ([GitLab][8])
* **`/opt`**: treat as mutable-by-need. Bluefin explicitly makes `/opt` writable by symlinking `/opt → /var/opt`. ([DeepWiki][9])

  * Copy that trick early. It prevents surprising breakage with software that still assumes `/opt` is writable.

---

# 2) Repo blueprint mirroring Bluefin patterns

Below is a “Bluefin-shaped” repo that is still beginner-friendly. It starts as one repo, but is designed so you can later split shared bits into separate OCI layers (like Bluefin’s `projectbluefin/common`). ([DeepWiki][2])

## Recommended folder layout

```text
your-distro/
  Containerfile
  Justfile
  image-versions.yml

  build_files/
    shared/
      build.sh                # orchestrator: runs numbered scripts in order
      clean-stage.sh          # optional cleanup helpers
      lib/*.sh                # reusable helpers (logging, retry, etc.)
    base/
      00-*.sh                 # base image steps
      10-*.sh
      20-tests.sh             # validation suite
    desktop/                  # optional: DE-specific steps (GNOME/KDE/etc.)
      00-*.sh
    dx/                       # optional: dev-experience variant
      00-*.sh

  system_files/
    shared/                   # “common” configs (can later move to a common OCI image)
      usr/etc/...
      usr/lib/systemd/system/...
      etc/containers/policy.json ...
    flavor-main/
      ...
    flavor-nvidia/
      ...

  just/                       # user-facing ujust commands (installed into /usr/share/ublue-os/just/)
    00-entry.just
    system.just
    update.just
    dev.just

  disk_config/
    image.toml                # user accounts, disk layout for BIB images
    iso.toml                  # installer ISO customization (bootc switch, post scripts)

  .github/workflows/
    build.yml                 # build+push OCI images
    build-disk.yml            # qcow2/raw/iso artifacts via bootc-image-builder

  hack/
    scripts/                  # misc helpers, smoke tests, release tooling
```

### Why this mirrors Bluefin well

* Bluefin organizes its build logic as **a mounted build context at `/ctx`** containing `build_files/` and `system_files/`, and runs a shared build orchestrator that executes **numbered scripts sequentially**. ([DeepWiki][9])
* Bluefin’s Containerfile is intentionally multi-stage: it imports external OCI dependencies, assembles a unified context, then runs the build. ([DeepWiki][10])
* Bluefin uses validation tests at the end of the pipeline to prevent “broken images” from shipping. ([DeepWiki][11])

---

## Naming + versioning scheme

### Image naming

Copy Bluefin’s mental model:

* **Image** = product/edition (e.g. `your-distro`, `your-distro-dx`)
* **Tag** = release stream (e.g. `stable`, `beta`, `latest`)
* **Flavor** = hardware/kernel variants (e.g. `main`, `nvidia`, `hwe`, `hwe-nvidia`)

Bluefin’s local build UX literally encodes this as `just build/run image tag flavor`. ([docs.projectbluefin.io][12])

### Versioning (human + machine)

Do what Bluefin’s bootc status output suggests: **FedoraMajor.YYYYMMDD.Build**.

Example: `42.20260128.0`

Bluefin uses this style in practice (their docs show “Image version: 40.20241101.0”). ([docs.projectbluefin.io][7])

### Tags you should support early

* Moving tag: `:stable` (latest stable)
* Date pin: `:stable-YYYYMMDD` (great for “pin to known good” debugging)
* Major pin: `:42` (explicit upgrade cycle control)

Bluefin documents these stream switching patterns using `bootc switch`. ([docs.projectbluefin.io][7])

---

# 3) Containerfile, build stages, and dependency pinning

## Containerfile stage pattern (Bluefin style)

Model your Containerfile as:

1. **common** (optional now, valuable later): pull a “common config” OCI image by digest, used only as a file source
2. **brew** (optional): pull a “brew integration” OCI image by digest, used only as a file source
3. **ctx**: assemble a single `/ctx` tree from repo files + common/brew files
4. **base**: `FROM ${BASE_IMAGE}:${FEDORA_MAJOR_VERSION}` (or other base), then run build pipeline with cache mounts + bind-mount `/ctx`, finish with `bootc container lint`

Bluefin’s documented stages are `common`, `brew`, `ctx`, `base`, and it uses BuildKit cache mounts and bind mounts to keep the context immutable while speeding builds. ([DeepWiki][10])

## `image-versions.yml` (digest pinning) is non-negotiable

Bluefin pins certain dependency images by digest (not tags) and injects them into `FROM …@sha256:…` at build time. This prevents tag-mutation supply chain attacks and improves reproducibility. ([DeepWiki][2])

Important nuance Bluefin calls out: they intentionally keep the **base Silverblue** pull tag-based (by Fedora major) to track upstream updates, while pinning other dependencies. ([DeepWiki][9])

### Practical recommendation for you

* **Pin by digest**: “your common config layer”, “brew integration layer”, any third-party layers you copy from.
* **Track by tag**: the upstream OS base (Fedora major tag), until you’re ready to fully own updates.

## Final image requirements checklist

Bluefin’s final Containerfile behavior is a great baseline:

* Make `/opt` writable via `/var/opt` symlink ([DeepWiki][9])
* `CMD /sbin/init` so the image boots systemd ([DeepWiki][9])
* Run `bootc container lint` in the image build to validate bootable-container requirements ([DeepWiki][9])

---

# 4) Local dev loop (fast, boring, repeatable)

## Tooling you need locally

* **podman** (and buildah/buildkit features)
* **just** (task runner)
* **qemu/libvirt** for VM boots (optional but strongly recommended)
* **bootc-image-builder** (run as a container)

Bluefin explicitly uses `just` for local development tasks. ([docs.projectbluefin.io][12])

## Minimal local commands (copy Bluefin’s ergonomics)

Examples straight from Bluefin’s local build doc:

```bash
# build an image (defaults to "latest" main)
just build your-distro

# build a specific stream/variant
just build your-distro stable main

# run the built container (debug shell inside image)
just run your-distro stable
```

Bluefin shows the general pattern and example invocations. ([docs.projectbluefin.io][12])

## Disk/ISO testing loop (qcow2/raw/installer ISO)

Universal Blue’s image-template provides an out-of-the-box workflow for producing qcow/raw/ISO using **bootc-image-builder**. ([GitHub][13])

Bootc-image-builder supports image types including `qcow2`, `raw`, and `bootc-installer` ISO. ([Osbuild][14])

**Local workflow recommendation:**

1. Build your OCI image locally.
2. Run bootc-image-builder in a privileged container to generate a qcow2.
3. Boot qcow2 in qemu to smoke test.

Note: bootc-image-builder examples run privileged and often use `--security-opt label=type:unconfined_t` to avoid SELinux labeling fights when mounting container storage. ([GitHub][15])

## Upgrade/rollback validation (Chromebook-reliability drills)

On a booted VM/system:

* **Check status**: `sudo bootc status` (Bluefin docs show exactly what to look for). ([docs.projectbluefin.io][7])
* **Rebase to your image**: `sudo bootc switch ghcr.io/<you>/<image>` (image-template docs). ([GitHub][13])
* **Rollback**: `bootc rollback` queues the previous deployment for next boot. ([GitLab][8])
* **Know the /etc rule**: `/etc` changes do not carry over on rollback; stash important config in `/var` when debugging. ([GitLab][8])

If you want to enforce trust like Bluefin recommends, use `bootc switch … --enforce-container-sigpolicy`. ([docs.projectbluefin.io][7])

---

# 5) CI/CD pipeline plan (GitHub Actions)

You want two primary workflows, mirroring the image-template approach:

## Workflow A: `build.yml` (OCI image manufacturing)

Goals:

1. Build OCI image(s) for your matrix (stream, flavor, edition)
2. Push to GHCR
3. Sign with cosign
4. Generate SBOM + attach attestation
5. Optional: run validation tests inside the built image, and fail fast

Why this matches the ecosystem:

* image-template’s `build.yml` publishes your OCI image to GHCR by default. ([GitHub][13])
* Bluefin’s pipeline is built around strong verification, digest pinning, and end-of-build validation tests. ([DeepWiki][2])

### Signing strategy

**Recommended default in 2026:** keyless signing via GitHub OIDC (no long-lived private key secret). Cosign supports this workflow; GitHub has documented keyless signing patterns. ([Chainguard Academy][16])

Also copy Bluefin’s idea of verifying upstream dependencies before building (they have `cosign verify-container` steps in their Justfile). ([GitHub][17])

### SBOM strategy

Use Syft to generate SBOMs and attach them as attestations (Syft explicitly documents `syft attest` working with cosign). ([GitHub][18])

## Workflow B: `build-disk.yml` (bootable artifacts)

Goals:

* Produce `qcow2`, `raw`, and optionally `bootc-installer` ISO from images published in Workflow A
* Upload as workflow artifacts (and optionally to S3, as the template supports)

This is exactly what image-template describes: `build-disk.yml` uses bootc-image-builder, and you customize `disk_config/iso.toml` to point to your image. ([GitHub][13])

If you want a polished approach, Bluefin LTS uses the **osbuild/bootc-image-builder-action** to generate ISOs from a matrix of variants. ([DeepWiki][19])

### Disk config files you’ll maintain

* `disk_config/image.toml`: user accounts + filesystem layout for VM/disk images (bootc-image-builder consumes it). ([DeepWiki][20])
* `disk_config/iso.toml`: installer behavior, kickstart post scripts, and switching installed system to your container image. ([DeepWiki][20])

---

# 6) Go automation plan (safe introduction, no big-bang rewrites)

The golden rule: **keep the “in-image provisioning” as shell** for a long time. Replace orchestration first.

Why: the hard parts (dnf/rpm-ostree ops, kernel/akmods prep, SELinux contexts) are easier to express and debug in shell scripts that run inside the container build. Bluefin’s entire build pipeline is script-driven for a reason. ([DeepWiki][9])

## Phase 1: Go wrapper around existing Justfile/shell

### Goal

Create a `yourctl` (or `distroctl`) Go CLI that:

* Validates arguments (image/tag/flavor)
* Sets env vars
* Executes `just build …`, `just build-iso …`, etc.
* Captures logs in a consistent format
* Optionally writes a build manifest JSON for later phases

### Why it’s safe

You keep Justfile as source of truth, but your team starts using `yourctl` immediately. Zero disruption.

### Exit criteria

* `yourctl build your-distro stable main` works locally and in CI (it calls `just build …`)
* `yourctl disk qcow2` produces a qcow2 via existing recipes
* All failures bubble up with actionable error messages (command + exit code + last N lines)

### Go libraries (Phase 1)

* CLI: `github.com/spf13/cobra`
* Exec: standard `os/exec` + context timeouts
* Logging: standard `log/slog`
* Small UX: `github.com/mattn/go-isatty` (optional)

## Phase 2: Go-native modules (templating, version resolution, orchestration)

### Goal

Start moving the “logic glue” from Just into Go, while still calling `podman`, `cosign`, `syft`, and `bootc-image-builder` binaries.

Implement these internal packages:

1. **Config loading**

   * Parse `image-versions.yml`
   * Parse your build matrix (YAML or JSON)
   * Validate combos like Bluefin’s `validate` task does ([docs.projectbluefin.io][12])

2. **Version resolution**

   * Compute `IMAGE_VERSION = FedoraMajor.YYYYMMDD.N`
   * Write `os-release` or labels (depending on your approach)
   * Emit a machine-readable manifest for CI artifacts

3. **Artifact orchestration**

   * Build OCI
   * Optional “rechunk” step (Bluefin has this optimization knob) ([docs.projectbluefin.io][12])
   * Push + sign + SBOM attest
   * Disk artifacts via bootc-image-builder

4. **Supply chain gates**

   * Verify pinned dependency images are signed before use (like Bluefin’s verify-container flow) ([DeepWiki][2])

### Exit criteria

* You can run the whole pipeline with `yourctl` even if `just` is removed from the host
* Yourctl emits a single `build-manifest.json` that CI uploads and that you can use for release notes

### Go libraries (Phase 2)

* YAML: `gopkg.in/yaml.v3`
* Templating: `text/template` + `github.com/Masterminds/sprig/v3` (optional helper funcs)
* Semver/date helpers: `github.com/Masterminds/semver/v3` (if you add semver streams)
* OCI inspection (digests, manifests): `github.com/google/go-containerregistry`
* Concurrency: `golang.org/x/sync/errgroup`

## Phase 3: Replace Just recipes with a Go CLI surface (optional)

### Goal

Make Go the UX contract:

* `yourctl build`
* `yourctl push`
* `yourctl sign`
* `yourctl sbom`
* `yourctl disk qcow2|raw|iso`
* `yourctl vm run`
* `yourctl upgrade-test` (spin VM, apply update, rollback, report)

### What still stays shell (for practicality)

* `build_files/**` scripts that run inside the Containerfile build (package install, system config)
* Any “distro plumbing” that is best expressed as POSIX glue (tiny systemd unit enablement scripts, etc.)
* bootc-image-builder invocation can remain a subprocess; it’s already a robust tool with many target types ([Osbuild][14])

### Exit criteria

* Your Justfile becomes thin aliases (or disappears)
* New contributors only need to learn `yourctl` commands

---

# 7) Minimal viable first release (MVP)

## Pick your MVP target

### Option A (fastest): developer workstation variant

* Base: `silverblue-main` (GNOME)
* Add: a minimal package set + your branding + 1–2 ujust commands
* Output: signed OCI + qcow2

### Option B (simplest): headless host

* Base: a bootc reference base image
* Add: sshd, podman, basic observability
* Output: signed OCI + raw disk image

Given your “Chromebook-like reliability” goal, I’d start with **Option A** because it forces you to solve the desktop-specific realities early (graphics, portals, Flatpak expectations), but still keeps the system atomic.

## MVP must-have checklist

### Bootability + update hygiene

* `bootc container lint` passes in the Containerfile build ([DeepWiki][9])
* `/opt → /var/opt` symlink (avoids surprising immutability breakage) ([DeepWiki][9])
* A clear update story:

  * auto-update timer choice (on/off by default)
  * documented rollback procedure (`bootc rollback`) ([GitLab][8])

### User + install experience

* `disk_config/image.toml` sets a default user for qcow2 testing ([DeepWiki][20])
* `disk_config/iso.toml` points installer to your image for “real installs” later ([DeepWiki][20])

### Supply chain trust (don’t postpone this too long)

* Image signing (cosign keyless OIDC preferred) ([Chainguard Academy][16])
* Enforce signature policy in docs (`bootc switch … --enforce-container-sigpolicy`) ([docs.projectbluefin.io][7])
* SBOM attestation via Syft + cosign ([GitHub][18])

### A tiny ujust surface area

Ship 2–4 curated commands for the first release (don’t boil the ocean). BlueBuild’s justfiles module explains how recipes become available as `ujust` on UBlue-derived images (or `blujust` elsewhere). ([BlueBuild][21])

Examples:

* `ujust update` (show status, trigger update)
* `ujust rollback`
* `ujust diagnostics` (collect logs, show bootc status)
* `ujust dev` (optional dev tooling toggle)

---

# 8) Risks + tradeoffs (and how to avoid dead ends)

## SELinux + privileged build tooling

* bootc-image-builder commonly runs privileged and may require SELinux label adjustments (`unconfined_t`) when mounting container storage. If you fight this too early, you’ll lose days. Follow the established invocation patterns first. ([GitHub][15])

**Avoidance tactic:** keep disk-image generation in CI initially (where you control the runner), then make local disk builds optional.

## /etc rollback surprises

Bootc rollbacks do not preserve `/etc` edits; `/etc` reverts to the older deployment. The docs explicitly recommend copying important modified configs into `/var` when debugging. ([GitLab][8])

**Avoidance tactic:** treat `/etc` as “override layer”, but store *state* and “must survive rollback” configs in `/var` (or generate them at boot via systemd units).

## Kernel modules, akmods, and proprietary drivers

On immutable systems, runtime compilation is a trap. Bluefin solves this by precompiling AKMODs at build time and pinning kernel versions for compatibility. ([DeepWiki][6])

**Avoidance tactic:** for MVP, ship **main** flavor only (no NVIDIA). Add NVIDIA after:

1. you have CI building reliably
2. you have a kernel strategy (pin/gate)
3. you understand module signing implications (secure boot)

## Third-party repos and package injection

Bluefin documents a secure strategy to prevent COPR repos from injecting fake versions of Fedora packages, by separating trusted Fedora installs from third-party repo enables. ([DeepWiki][22])

**Avoidance tactic:** in your build scripts, keep a strict “Fedora first, third-party second” installation order, and prefer digest-pinned image layers for big integrations when possible.

## “Too many variants” too early

Bluefin has streams, gated kernels, DX variants, HWE, NVIDIA, etc. That’s awesome, but it’s also a complexity multiplier. ([docs.projectbluefin.io][7])

**Avoidance tactic:** MVP supports exactly:

* 1 edition (base)
* 1 stream (`stable`)
* 1 flavor (`main`)
  Everything else is a milestone, not a starting line.

---

# 9) Ordered timeline with milestones and exit criteria

## Milestone 0: Repo skeleton + base selection (Day 1–2)

**Do:**

* Fork/start from image-template for immediate bootc correctness, then reshape into the Bluefin-like layout above. ([docs.projectbluefin.io][23])
* Choose base: `silverblue-main`

**Exit criteria:**

* Repo builds an OCI image locally (even if unchanged)
* `bootc container lint` passes in Containerfile build ([DeepWiki][9])

---

## Milestone 1: MVP image boots in a VM (Day 3–5)

**Do:**

* Add 1–3 packages, 1 config file overlay, and a basic test script.
* Generate qcow2 using bootc-image-builder and boot it.

**Exit criteria:**

* qcow2 boots to a login screen (or ssh prompt)
* `bootc status` works in the VM ([docs.projectbluefin.io][7])

---

## Milestone 2: Add Bluefin-style build pipeline structure (Week 2)

**Do:**

* Move logic into `build_files/shared/build.sh` orchestrating numbered scripts (base + optional desktop). ([DeepWiki][9])
* Add `system_files/` overlays and copy logic.

**Exit criteria:**

* Repeated builds are deterministic (same inputs produce same result)
* A failing script fails the build fast (like Bluefin’s `set -eoux pipefail` discipline) ([DeepWiki][11])

---

## Milestone 3: Dependency pinning + verification (Week 2–3)

**Do:**

* Introduce `image-versions.yml` and digest-pinned sources for any copied layers. ([DeepWiki][2])
* Add cosign verification step for pinned images.

**Exit criteria:**

* CI fails if digest-pinned dependencies are unsigned/tampered
* Updating a digest is a deliberate PR (Renovate-style automation later) ([DeepWiki][2])

---

## Milestone 4: CI builds + signs + SBOM attests (Week 3)

**Do:**

* GitHub Actions `build.yml`: build matrix (even if only 1 combo), push to GHCR, keyless sign, SBOM attest. ([GitHub][13])

**Exit criteria:**

* `ghcr.io/you/your-distro:stable` exists
* Signature present, SBOM attestation present
* You can `bootc switch … --enforce-container-sigpolicy` successfully ([docs.projectbluefin.io][7])

---

## Milestone 5: CI disk artifacts (Week 4)

**Do:**

* GitHub Actions `build-disk.yml`: build qcow2/raw/ISO using bootc-image-builder. ([Osbuild][14])

**Exit criteria:**

* Workflow uploads qcow2 + ISO artifacts
* ISO installs and boots on at least one test VM

---

## Milestone 6: Go automation Phase 1 (Week 4–5)

**Do:**

* Implement `yourctl` wrapper calling `just` targets.

**Exit criteria:**

* `yourctl build`, `yourctl disk iso`, `yourctl vm run` all work locally
* CI can call `yourctl` instead of `just`

---

## Milestone 7: Go automation Phase 2 (Weeks 6–8)

**Do:**

* Move matrix validation, version computation, and artifact orchestration into Go.
* Keep actual provisioning scripts as shell.

**Exit criteria:**

* `yourctl pipeline` runs end-to-end without relying on justfile logic
* Build manifest JSON is generated and attached to releases

---

## Milestone 8: Variants (after you’re stable)

Pick one:

* DX variant
* NVIDIA flavor (requires kernel/akmods strategy) ([DeepWiki][6])
* Multiple streams (stable/beta/latest)

**Exit criteria:**

* Each new variant has:

  * a tested VM boot path
  * upgrade + rollback verification
  * CI signing + SBOM parity

---

# First 7 days task list (doable, concrete)

1. **Create repo from image-template** and set base to `silverblue-main` (or Bluefin directly if you want). ([docs.projectbluefin.io][23])
2. **Add a single visible customization** (hostname default, a wallpaper, a tiny `/usr/etc` file) via `system_files/`.
3. **Implement `build_files/shared/build.sh`** that runs one numbered script (`00-packages.sh`) and one test (`20-tests.sh`). ([DeepWiki][9])
4. **Add `bootc container lint`** to your Containerfile final stage (if not already present) and ensure it passes. ([DeepWiki][9])
5. **Generate a qcow2** with bootc-image-builder and boot it in qemu. ([Osbuild][14])
6. Inside the VM: run **`sudo bootc status`**, then perform a **rollback drill** (`bootc rollback`, reboot, confirm), and note the `/etc` behavior. ([GitLab][8])
7. **Wire up GitHub Actions** to build and push `:stable` to GHCR, then add cosign signing + Syft SBOM attestation. ([GitHub][13])

If you execute just those seven, you’ll have a booting, updateable, rollbackable, publishable “mini-distro” by the end of the week. Then the Go automation work becomes a pleasant engineering project, not a rescue mission 🛠️🐧

[1]: https://docs.fedoraproject.org/en-US/bootc/getting-started/?utm_source=chatgpt.com "Getting Started with Bootable Containers - Fedora Docs"
[2]: https://deepwiki.com/ublue-os/bluefin/2.3-containerfile-and-multi-stage-build "Image Dependency Pinning (image-versions.yml) | ublue-os/bluefin | DeepWiki"
[3]: https://blue-build.org/learn/universal-blue/?utm_source=chatgpt.com "Building on Universal Blue - BlueBuild"
[4]: https://universal-blue.org/?utm_source=chatgpt.com "Universal Blue – Powered by the future, delivered today"
[5]: https://docs.projectbluefin.io/blog/modernizing-custom-images/ "Modernizing custom images based on Bluefin | Bluefin"
[6]: https://deepwiki.com/ublue-os/bluefin/2.5-build-scripts-and-stages "Kernel and AKMOD Management | ublue-os/bluefin | DeepWiki"
[7]: https://docs.projectbluefin.io/administration/ "Administrator's Guide | Bluefin"
[8]: https://fedora.gitlab.io/bootc/docs/bootc/auto-updates/ "Auto-Updates and Manual Rollbacks :: Local Preview"
[9]: https://deepwiki.com/ublue-os/bluefin/2.1-image-version-management "Containerfile and Multi-Stage Build Process | ublue-os/bluefin | DeepWiki"
[10]: https://deepwiki.com/ublue-os/bluefin/2.1-build-orchestration-with-justfile "Containerfile and Multi-Stage Build Process | ublue-os/bluefin | DeepWiki"
[11]: https://deepwiki.com/ublue-os/bluefin/2.7-image-rechunking-and-ostree-optimization "Build Testing and Validation | ublue-os/bluefin | DeepWiki"
[12]: https://docs.projectbluefin.io/local/ "Building Locally | Bluefin"
[13]: https://github.com/ublue-os/image-template "GitHub - ublue-os/image-template: Build your own custom Universal Blue Image!"
[14]: https://osbuild.org/docs/bootc/ "bootc-image-builder | Image Builder"
[15]: https://github.com/osbuild/bootc-image-builder "GitHub - osbuild/bootc-image-builder: A container for deploying bootable container images."
[16]: https://edu.chainguard.dev/open-source/sigstore/how-to-keyless-sign-a-container-with-sigstore/?utm_source=chatgpt.com "How to Keyless Sign a Container Image with Sigstore"
[17]: https://github.com/ublue-os/bluefin/blob/main/Justfile?utm_source=chatgpt.com "bluefin/Justfile at main · ublue-os/bluefin · GitHub"
[18]: https://github.com/anchore/syft/wiki/Attestation?utm_source=chatgpt.com "Attestation · anchore/syft Wiki · GitHub"
[19]: https://deepwiki.com/ublue-os/bluefin-lts/6.4-iso-and-bootable-media-generation?utm_source=chatgpt.com "ISO and Bootable Media Generation | ublue-os/bluefin-lts | DeepWiki"
[20]: https://deepwiki.com/ublue-os/bluefin-lts/8.3-iso-and-installer-customization "ISO and Installer Customization | ublue-os/bluefin-lts | DeepWiki"
[21]: https://blue-build.org/reference/modules/justfiles/?utm_source=chatgpt.com "justfiles | BlueBuild"
[22]: https://deepwiki.com/ublue-os/bluefin/2.4-build-orchestration-with-justfile "Package Installation and Security | ublue-os/bluefin | DeepWiki"
[23]: https://docs.projectbluefin.io/tips/ "Tips and Tricks | Bluefin"
