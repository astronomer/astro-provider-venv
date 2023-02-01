# Astro Runtime Docker Frontend

This custom Docker BuildKit frontend (see this [blog]( https://www.docker.com/blog/compiling-containers-dockerfiles-llvm-and-buildkit/) for details)
adds a new custom command `PYENV` that can be used inside Dockerfiles to install new Python versions
and virtual environments with custom dependencies.

## Syntax

To use the custom frontend set the following somewhere in your Dockerfile:

```
# syntax=astronomer/astro-runtime-frontend
```

The `PYENV` command has the following syntax:
```
PYENV <python-version> <venv-name> [<reqs-file>]
```

For example:

```
PYENV 3.8 snowpark-env reqs/snowpark-reqs.txt
```

This will install Python 3.8 into the image from the upstream `python:3.8-slim` image and create
a virtual environment at `/home/astro/.venv/snowpark-env`. It will also run pip install
with the requirements it finds in the `reqs/snowpark-reqs.txt` file in the docker context.

The requirements file is optional, so one can install a bare Python environment with something like:

```
PYENV 3.10 venv1
```

## Building

To test the custom frontend you can build it locally using `make image-build` from the root of the
project directory. Then run `docker build` on a Dockerfile containing the custom syntax.

**Note**: If buildkit is not enabled by default you should run `docker buildx build`.

## Using the built image

See the `example` sub-directory for an example setup, which contains the Airflow "magic" to do the
translation. After building the custom frontend and replacing the value of `AIRFLOW_CONN_SNOWFLAKE_DEFAULT`:

```
astro dev start
```

## Current limitations

* The Python version specified must be available as an official python slim image
  (the `-slim` suffix) is automatically added.
* The base image in the first `FROM` in the Dockerfile must be an astro-runtime **base** image, e.g. `quay.io/astronomer/astro-runtime:7.0.0-base`.
* The docker context must have `packages.txt` and `requirements.txt` files at its root (these can be empty).
* Currently only works in standalone mode or `astro dev`.
* Currently only support the `PythonOperator`.
