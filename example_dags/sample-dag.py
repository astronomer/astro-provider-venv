from airflow.decorators import dag, task
from pendulum import datetime


@dag(
    start_date=datetime(2023, 1, 1),
    schedule=None,
    tags=["example", "astronomer"],
)
def multi_python_easy_venv():
    @task
    def print_python():
        import sys

        print(f"My python version is {sys.version}")

        return sys.executable

    @task.venv("my-venv")
    def other_python():
        import sys

        print(f"My python version is {sys.version}")

        return sys.executable

    @task
    def analyze(local_ver, remote_ver):
        print(f"Pythons used were {local_ver, remote_ver}")

    analyze(print_python(), other_python())


multi_python_easy_venv()
