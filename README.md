<p align="center">
  <a href="https://www.airflow.apache.org">
    <img alt="Airflow" src="https://cwiki.apache.org/confluence/download/attachments/145723561/airflow_transparent.png?api=v2" width="60" />
  </a>
</p>
<h1 align="center">
  Apache Airflow virtual envs made easy
</h1>
<h3 align="center">
  Making it easy to run tasks in isolated python virtual environments (venv) in Dockerfiles.
  Maintained with ❤️ by Astronomer.
</h3>

Let's say you want to be able to run an Airflow task against Snowflake's Snowpark -- which requires Python 3.8.

With the addition of the ExternalPythonOperator in Airflow 2.4 this is possible, but managing the build process to get clean, quick Docker builds can take a lot of plumbing.

This repo provides a nice packaged solution to it, that plays nicely with Docker image caching.

## Synopsis

### Create a requirements.txt file

For example, `snowpark-requirements.txt`

```
snowflake-snowpark-python[pandas]

# To get credentials out of a connection we need these in the venv too sadly
apache-airflow
psycopg2-binary
apache-airflow-providers-snowflake
```

### Use our custom Docker build frontend

```Dockerfile
# syntax=quay.io/astronomer/airflow-extensions:v1

FROM quay.io/astronomer/astro-runtime:7.2.0-base

PYENV 3.8 snowpark snowpark-requirements.txt
```

Note: That first `# syntax=` comment is important, don't leave it out!

Read more about the [new `PYENV` instruction](#pyenv-docker-instruction)

### Use it in a DAG

```python
from __future__ import annotations

import sys

from airflow import DAG
from airflow.decorators import task
from airflow.utils.timezone import datetime

with DAG(
    dag_id="astro_snowpark",
    schedule=None,
    start_date=datetime(2022, 1, 1),
    catchup=False,
    tags=["example"],
) as dag:

    @task
    def print_python():
        print(f"My python version is {sys.version}")

    @task.venv("snowpark")
    def snowpark_task():
        from airflow.providers.snowflake.hooks.snowflake import SnowflakeHook
        from snowflake.snowpark import Session

        print(f"My python version is {sys.version}")

        hook = SnowflakeHook("snowflake_default")
        conn_params = hook._get_conn_params()
        session = Session.builder.configs(conn_params).create()
        tables = session.sql("show tables").collect()
        print(tables)

        df_table = session.table("sample_product_data")
        print(df_table.show())
        return df_table.to_pandas()

    @task
    def analyze(df):
        print(f"My python version is {sys.version}")
        print(df.head(2))

    print_python() >> analyze(snowpark_task())
```

## Requirements

This needs Apache Airflow 2.4+ for the [ExternalPythonOperator] to work.

## Requirements for building Docker images

This needs the [buildkit](https://docs.docker.com/build/buildkit/) backend for Docker.

It is enabled by default for Docker Desktop users; Linux users will need to enable it:

To set the BuildKit environment variable when running the docker build command, run:

```bash
DOCKER_BUILDKIT=1 docker build .
```

To enable docker BuildKit by default, set daemon configuration in /etc/docker/daemon.json feature to true and restart the daemon. If the daemon.json file doesn’t exist, create new file called daemon.json and then add the following to the file.

```json
{
  "features": {
    "buildkit" : true
  }
}
```

And restart the Docker daemon.

The syntax extension also currently expects to find a `packages.txt` and `requirements.txt` in the Docker context directory (these can be empty by default).

## Reference

### `PYENV` Docker instruction

The `PYENV` command adds a Python Virtual Environment, running on the specified Python version to the docker image, and optionally install packages from a requirements.txt

It has the following syntax:

```Dockerfile
PYENV <python-version> <venv-name> [<reqs-file>]
```

The requirements file is optional, so one can install a bare Python environment with something like:

```Dockerfile
PYENV 3.10 venv1
```

### `@task.venv` decorator

TODO! Write the decorator, then fill out docs!

## In This Repo

### [`buildkit/`](buildkit/)

This contains the cusotm  Docker BuildKit frontend (see this [blog]( https://www.docker.com/blog/compiling-containers-dockerfiles-llvm-and-buildkit/) for details) adds a new custom command `PYENV` that can be used inside Dockerfiles to install new Python versions and virtual environments with custom dependencies.

### [`provider/`](provider/)

This contains an Apache Airflow provider that providers the `@task.venv` decorator.

## The Gory Details

**a.k.a. How do I do this all manually?**

The `# syntax` line tells buildkit to user our Build frontend to process the Dockerfile into instructions.

The example Dockerfile above gets converted into roughly following instructions

```Dockerfile
USER root
COPY --link --from=python:3.8-slim /usr/local/bin/*3.8* /usr/local/bin/
COPY --link --from=python:3.8-slim /usr/local/include/python3.8* /usr/local/include/python3.8
COPY --link --from=python:3.8-slim /usr/local/lib/pkgconfig/*3.8* /usr/local/lib/pkgconfig/
COPY --link --from=python:3.8-slim /usr/local/lib/*3.8*.so* /usr/local/lib/
COPY --link --from=python:3.8-slim /usr/local/lib/python3.8 /usr/local/lib/python3.8
RUN /sbin/ldconfig /usr/local/lib
RUN ln -s /usr/local/include/python3.8 /usr/local/include/python3.8m

USER astro
RUN mkdir -p /home/astro/.venv/snowpark
COPY reqs/venv1.txt /home/astro/.venv/snowpark/requirements.txt
RUN /usr/local/bin/python3.8 -m venv --system-site-packages /home/astro/.venv/snowpark
ENV ASTRO_PYENV_snowpark /home/astro/.venv/snowpark/bin/python
RUN --mount=type=cache,target=/home/astro/.cache/pip /home/astro/.venv/snowpark/bin/pip --cache-dir=/home/astro/.cache/pip install -r /home/astro/.venv/snowpark/requirements.txt
```

The final part of this puzzle from the Airflow operator is to look up the path to `python` in the created venv using the `ASTRO_PYENV_*` environment variable:

```python
@task.external_python(python=os.environ["ASTRO_PYENV_snowpark"])
def snowpark_task():
    ...
```

[ExternalPythonOperator]: https://airflow.apache.org/docs/apache-airflow/stable/howto/operator/python.html#externalpythonoperator
