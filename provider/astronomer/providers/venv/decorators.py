from __future__ import annotations

import inspect
import os
import textwrap
from typing import Callable

from airflow.decorators.base import (
    DecoratedOperator,
    TaskDecorator,
    task_decorator_factory,
)
from airflow.operators.python import ExternalPythonOperator


class PythonVenvDecoratedOperator(DecoratedOperator, ExternalPythonOperator):
    custom_operator_name: str = "@task.venv"

    def __init__(self, *, python_callable, op_args, op_kwargs, **kwargs) -> None:
        kwargs_to_upstream = {
            "python_callable": python_callable,
            "op_args": op_args,
            "op_kwargs": op_kwargs,
        }
        super().__init__(
            kwargs_to_upstream=kwargs_to_upstream,
            python_callable=python_callable,
            op_args=op_args,
            op_kwargs=op_kwargs,
            **kwargs,
        )

    def get_python_source(self):
        from airflow.utils.decorators import remove_task_decorator

        raw_source = inspect.getsource(self.python_callable)
        res = textwrap.dedent(raw_source)
        res = remove_task_decorator(res, self.custom_operator_name)
        return res

    # Override the version from superclass to be tolerant of astro specific post releases.
    def _get_airflow_version_from_target_env(self) -> str | None:
        import subprocess

        import airflow
        from airflow.exceptions import AirflowConfigException
        from packaging.version import Version

        airflow_version = Version(airflow.__version__)
        try:
            result = subprocess.check_output(
                [self.python, "-c", "from airflow import __version__; print(__version__)"],
                text=True,
                # Avoid Airflow logs polluting stdout.
                env={
                    **os.environ,
                    "_AIRFLOW__AS_LIBRARY": "true",
                    "AIRFLOW__CORE__LAZY_LOAD_PROVIDERS": "True",
                    "AIRFLOW__CORE__LAZY_LOAD_PLUGINS": "True",
                },
            )
            target_airflow_version = Version(result.strip())
            if target_airflow_version.base_version != airflow_version.base_version:
                raise AirflowConfigException(
                    f"The version of Airflow installed for the {self.python}("
                    f"{target_airflow_version}) is different than the runtime Airflow version: "
                    f"{airflow_version}. Make sure your environment has the same Airflow version "
                    f"installed as the Airflow runtime."
                )
            return target_airflow_version
        except Exception as e:
            if self.expect_airflow:
                self.log.warning("When checking for Airflow installed in virtual environment got %s", e)
                self.log.warning(
                    f"This means that Airflow is not properly installed by  "
                    f"{self.python}. Airflow context keys will not be available. "
                    f"Please Install Airflow {airflow_version.base_version} in your environment to access them."
                )
            return None


def venv_task(
    venv: str,
    python_callable: Callable | None = None,
    **kwargs,
) -> TaskDecorator:
    """Wraps a callable into an Airflow operator to run via a Python virtual environment.

    Accepts kwargs for operator kwarg. Can be reused in a single DAG.

    :param python: Full path string (file-system specific) that points to a Python binary inside
        a virtualenv that should be used (in ``VENV/bin`` folder). Should be absolute path
        (so usually start with "/" or "X:/" depending on the filesystem/os used).
    :param python_callable: Function to decorate
    :param multiple_outputs: If set to True, the decorated function's return value will be unrolled to
        multiple XCom values. Dict will unroll to XCom values with its keys as XCom keys.
        Defaults to False.
    """

    task_venv = os.environ.get(f"ASTRO_PYENV_{venv}")
    if not task_venv:
        raise ValueError(f"invalid virtual environment: {venv!r} (missing ASTRO_PYENV_{venv})")
    return task_decorator_factory(
        python=task_venv,
        python_callable=python_callable,
        decorated_operator_class=PythonVenvDecoratedOperator,
        **kwargs,
    )
