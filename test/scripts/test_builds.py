import imgtestlib as testlib


def test_builds(distro, arch, imgtype, tmp_path):
    """
    Build all the manifests generated using the config-list for the specified distro and arch.
    The distro, arch, and imgtype arguments are set using command line args when calling pytest (see conftest.py).
    """
    manifests_path = tmp_path / "manifests"

    testlib.gen_manifests(str(manifests_path), distros=[distro], arches=[arch], images=[imgtype])
    manifests = testlib.read_manifests(str(manifests_path))

    # let's go looking for them manifests
    build_requests = testlib.filter_builds(manifests, distro=distro, arch=arch)
    if not build_requests:
        print("No images to build.")
        return

    print(f"Will build {len(build_requests)} image{"s" if len(build_requests) > 1 else ""}")
    for n, item in enumerate(build_requests, start=1):
        distro = item["distro"]
        arch = item["arch"]
        image_type = item["image-type"]
        config_name = item["config"]["name"]
        print(f"{n}: {distro} {arch} {image_type} {config_name}")
