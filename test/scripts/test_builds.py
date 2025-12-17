import pytest


def test_build_request(build_request):
    assert build_request
    print("Build request ok!!")


def test_manifest(manifest):
    manifest = manifest["manifest"]
    assert manifest["version"] == "2"
    assert "sources" in manifest
    assert "pipelines" in manifest
    assert manifest["pipelines"][0]["name"] == "build"


def test_build(image):
    image_path = image["image-path"]
    assert image_path


def test_boot(image):
    image_path = image["image-path"]
    assert image_path

    request = image["build-request"]
    imgtype = request["image-type"]
    pytest.skip(f"{imgtype} boot test not supported")
