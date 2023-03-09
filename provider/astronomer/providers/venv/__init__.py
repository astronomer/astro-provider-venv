__version__ = "1.0.0a1"


def get_provider_info():
    return {
        "package-name": "airflow-provider-venv",
        "name": "airflow-provider-venv",
        "description": "Easily create and use Python Virtualenvs in Apache Airflow",
        "task-decorators": [
            {
                "name": "venv",
                "class-name": "astronomer.providers.venv.decorators.venv_task",
            },
        ],
    }
