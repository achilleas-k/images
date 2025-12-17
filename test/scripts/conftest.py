import tempfile

import pytest

import imgtestlib as testlib


def name_from_build_request(request):
    """
    Wrapper for testlib.gen_build_name() that uses a buid_request structure instead of individual arguments.
    """
    return testlib.gen_build_name(request["distro"], request["arch"], request["image-type"], request["config"]["name"])


@pytest.fixture(scope="session", params=testlib.gen_build_requests())
def build_request(request):
    """
    Pytest fixture that returns one test build request.

    Each build request has the following structure:
    {
        "distro": "",
        "arch": "",
        "image-type": "",
        "repositories": [],
        "config": {
            "name": "",
        },
    }
    """
    return request.param


@pytest.fixture(scope="session")
def manifest(build_request, tmp_path_factory):
    """
    Pytest fixture that generates a full manifest based on a build request (see build_request fixture) and returns a
    dictionary that includes the build_request, the manifest ID, and the manifest itself.

    The dictionary has the following structure:
    {
        "build-request": "",
        "manifest-id": "",
        "manifest": {}
    }
    """
    manifest_dir = tmp_path_factory.mktemp("manifest")
    testlib.gen_manifest(build_request, manifest_dir)
    filename = name_from_build_request(build_request) + ".json"
    manifest_path = manifest_dir / filename
    with manifest_path.open(encoding="utf-8") as manifest_fp:
        manifest_data = json.load(manifest_fp)

    manifest_id = testlib.get_manifest_id(manifest_data)

    return {
        "build-request": build_request,
        "manifest": manifest_data,
        "manifest-id": manifest_id,
    }


@pytest.fixture(scope="session")
def build_cache(tmp_path_factory):
    """
    Pytest fixture that downloads the info.json and bootc-image-builder image ID (bib*) files from the test cache.
    """
    build_cache_path = tmp_path_factory.mktemp("cache")
    testlib.dl_build_info(build_cache_path)
    return build_cache_path


@pytest.fixture(scope="session")
def cached_image(manifest, build_cache):
    """
    Pytest fixture that fetches an image from the cache based on the manifest. Returns the same data as the manifest
    fixture with an extra key, "image-path", which contains the path to the image that was downloaded.

    If the image is not available in the cache, the "image-path" is None.
    """
    build_request = manifest["build-request"]
    distro = build_request["distro"]
    arch = build_request["arch"]
    manifest_id = manifest["manifest-id"]

    cached_element_dir = build_cache / testlib.gen_build_info_dir_path_prefix(distro, arch, manifest_id)
    build_info_path = cached_element_dir / "info.json"
    if not build_info_path.exists():
        print(f"Cached image for {manifest_id} not found.")
        manifest["image-path"] = None
        return manifest

    with build_info_path.open(encoding="utf-8") as build_info_fp:
        build_info = json.load(build_info_fp)

    commit = build_info["commit"]
    pr = build_info.get("pr")
    url = f"https://github.com/osbuild/images/commit/{commit}"
    print(f"🖼️ Manifest {manifest_id} was successfully built in commit {commit}\n  {url}")
    if "gh-readonly-queue" in pr:
        print(f"  This commit was on a merge queue: {pr}")
    elif pr:
        print(f"  PR-{pr}: https://github.com/osbuild/images/pull/{pr}")
    else:
        print("  No PR/branch info available")

    build_info_dir = build_cache / distro / arch / f"manifest-id-{manifest_id}"

    # download the corresponding image
    testlib.dl_build_cache(build_cache, distro, arch, manifest_id)
    manifest["image-path"] = build_info_dir
    return manifest


def pytest_configure(config):
    config.addinivalue_line(
        "markers", "images_integration"
    )
