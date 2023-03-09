import sys
from pathlib import Path

import pytest
from airflow import DAG
from airflow.models import DagBag, DagRun


@pytest.fixture
def in_memory_airflow_db(monkeypatch):
    monkeypatch.setenv("AIRFLOW__DATABASE__SQL_ALCHEMY_CONN", "sqlite://")
    from airflow.settings import initialize
    from airflow.utils.db import upgradedb

    initialize()
    upgradedb(reserialize_dags=False)


# To parse the dag, we need the venv variable set.
@pytest.mark.usefixtures("venv")
@pytest.fixture(scope="module")
def dagbag(venv):
    examples = Path(__file__).parents[1] / "example_dags"
    return DagBag(examples, include_examples=False)


@pytest.fixture(scope="module")
def example_dag(dagbag) -> "DAG":
    return dagbag.dags["multi_python_easy_venv"]


@pytest.fixture(scope="module")
def venv(tmp_path_factory) -> Path:
    import venv

    path = tmp_path_factory.mktemp("venv")

    venv.EnvBuilder().create(path)

    python = path / "bin" / "python"
    with pytest.MonkeyPatch.context() as monkeypatch:
        monkeypatch.setenv("ASTRO_PYENV_my-venv", str(python))
        yield python


def test_parse_example_dags(venv, dagbag):
    assert dagbag.import_errors == {}
    assert "multi_python_easy_venv" in dagbag.dags


@pytest.mark.usefixtures("in_memory_airflow_db")
def test_venv_dag(example_dag, venv):
    example_dag.test()
    dag_run = example_dag.get_last_dagrun()
    assert isinstance(dag_run, DagRun)
    other_python = dag_run.get_task_instance("other_python")
    print_python = dag_run.get_task_instance("print_python")
    assert other_python.xcom_pull(task_ids=other_python.task_id) == str(venv)

    assert print_python.xcom_pull(task_ids=print_python.task_id) == sys.executable
