__version__ = "1.0.0a2"


def get_provider_info():
    return {
        "package-name": "astro-provider-venv",
        "name": "astro-provider-venv",
        "description": "Easily create and use Python Virtualenvs in Apache Airflow",
        "task-decorators": [
            {
                "name": "venv",
                "class-name": "astronomer.providers.venv.decorators.venv_task",
            },
        ],
    }
