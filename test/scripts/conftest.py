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


def pytest_configure(config):
    config.addinivalue_line(
        "markers", "images_integration"
    )
