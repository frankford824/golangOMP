ARG PYTHON_BASE_IMAGE=python@sha256:2eb3cdf88d45b1c304d593dfe77f431e00ca13c6e707416a259ed3987d5c6aa8
FROM ${PYTHON_BASE_IMAGE}

ARG FIXTURE_SCRIPT_SHA256
LABEL org.opencontainers.image.title="yongbo-ab-upload-fixture" \
      com.yongbo.ab.fixture-script-sha256="${FIXTURE_SCRIPT_SHA256}"

COPY scripts/ab/ab_upload_fixture.py /opt/yongbo-ab/ab_upload_fixture.py
RUN python3 -m py_compile /opt/yongbo-ab/ab_upload_fixture.py

ENTRYPOINT ["python3", "/opt/yongbo-ab/ab_upload_fixture.py"]
