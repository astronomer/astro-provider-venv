import textwrap

from airflow.decorators import task
from airflow.operators.python import ExternalPythonOperator


def test_decorator(monkeypatch):

    monkeypatch.setenv("ASTRO_PYENV_myvenv", "/opt/venvs/myvenv/bin/python")

    @task.venv("myvenv")
    def my_task():
        ...

    operator = my_task().operator

    assert isinstance(operator, ExternalPythonOperator)
    assert operator.python == "/opt/venvs/myvenv/bin/python"
    assert operator.get_python_source() == textwrap.dedent(
        """\
        def my_task():
            ...
        """
    )
