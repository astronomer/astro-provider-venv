# Astro Runtime Docker Frontend

This custom Docker BuildKit frontend (see this [blog]( https://www.docker.com/blog/compiling-containers-dockerfiles-llvm-and-buildkit/) for details)
adds a new custom command `PYENV` that can be used inside Dockerfiles to install new Python versions
and virtual environments with custom dependencies.

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
