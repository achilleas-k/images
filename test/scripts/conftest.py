import json

import pytest

import imgtestlib as testlib


def pytest_addoption(parser):
    parser.addoption(
        "--persistent-cache-root",
        type=str,
        help=("store any generated data in the given path and persist after the tests are finished, "
              "instead of using tmpdirs and deleting them - this includes: rpmmd cache, build cache, osbuild store"),
    )
    parser.addoption(
        "--dry-run",
        action="store_true",
        default=False,
        help="don't build or boot any images, only print which ones would be tested (based on cache availability)",
    )
    parser.addoption(
        "--force-build",
        action="store_true",
        default=False,
        help="build matching images without checking the cache",
    )


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


# TODO: require root or passwordless sudo
@pytest.fixture(scope="session")
def image(cached_image, tmp_path_factory):
    """
    Pytest fixture that builds an image from a manifest fixture. Returns the same data as the manifest fixture with an
    extra key, "image-path", which contains the path to the image.

    If the image is available in the cache (via the cached_image fixture), it will not be rebuilt.
    """
    if cached_image["image-path"]:
        print("Cached image found: skipping build")
        return cached_image

    config_path = tmp_path_factory.mktemp("config") / "config.json"
    build_dir = tmp_path_factory.mktemp("image")

    # we want to reuse the metadata from cached_image, but let's rename it to 'new_image' for readability
    new_image = cached_image
    build_request = new_image["build-request"]
    manifest_id = new_image["manifest-id"]

    print(f"Building image image for {manifest_id}.")
    with config_path.open(mode="w", encoding="utf-8") as config_fp:
        json.dump(build_request["config"], config_fp)

    distro = build_request["distro"]
    arch = build_request["arch"]
    imgtype = build_request["image-type"]
    testlib.build_image(distro, arch, imgtype, config_path, build_dir)
    image_file = testlib.find_image_file(new_image["image-path"])
    new_image["image-path"] = image_file

    return new_image


def pytest_configure(config):
    config.addinivalue_line(
        "markers", "images_integration"
    )
