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
