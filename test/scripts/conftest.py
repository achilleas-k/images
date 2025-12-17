import tempfile

import pytest

import imgtestlib as testlib


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


def pytest_configure(config):
    config.addinivalue_line(
        "markers", "images_integration"
    )
