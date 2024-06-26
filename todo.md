# OTK INTEGRATION PLAN AND CHECKLIST

Initial transition

- [ ] Make `manifest.Manifest` an interface (with the existing `manifest.Manifest` the first implementation).
- [ ] Create a second implementation of the `Manifest` interface for OTK manifests.
- [ ] The `Manifest.Serialize()` method for otk manifests runs `otk compile` on the otk file (see [OTKSerialize() prototype](pkg/manifest/manifest.go#L183)).
- [ ] Create a new `distro.ImageType` implementation for otk-defined image types (see [otkImageType prototype](pkg/distro/fedora/otk.go#L18)).
- [ ] The `ImageType.Manifest()` method for otk image types prepares the otk `Manifest` object (see [otkImageType.Manifest() prototype](pkg/distro/fedora/otk.go#L151)).
    - `otk compile` can be run here instead of `Serialize()`.  It probably wont make much of a difference.
- [ ] Initialise image types by looking for files in a given directory tree (categorised?  perhaps `<distro>/<arch>/<imgtypename>.yaml`?).
    - [ ] Initialise an image type with the minimum information we need, perhaps based on variables in the omnifest itself.  
        - How much information do we need for each image type (in the Go code)?  Filename and mime type for example are important to know for handling the artifact, but we certainly don't need to know the ISOLabel.
        - We might need a schema for the values needed by osbuild-composer.
- [ ] Customizations: ???

Long term

- [ ] Make all image types otk types.  We can probably get away with one implementation of the `distro.ImageType` interface (or get rid of it entirely).
- [ ] Change the flow to reflect what's actually happening.
    - Currently, the flow is (very roughly):
        1. `imgType = <hardcoded image type configuration>`
        2. `manifest = imgType.Manifest(blueprint and options)`
        3. `packages = depsolve(manifest.Packages)`
        4. `containers = resolveContainers(manifest.Containers)`
        5. `ostreeCommits = resolveCommits(manifest.Commits)`
        6. `serializedManifest = manifest.Serialize(packages, containers, ostreeCommits)`
    - New flow will probably be something like:
        1. `imgType = otk.LoadMetadata(path)`  // load image type metadata from a discovered otk yaml file path
        2. `serializedManifest = imgType.Manifest(blueprint and options)`
    - There's probably no reason to break down step 2 into a config and serialize function.  Resource resolution will become part of the `otk compile` command.  Perhaps otk itself can run different externals asynchronously so container resolves don't have to wait for depsolves etc.
