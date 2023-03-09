import os

os.environ["AIRFLOW__CORE__EXECUTOR"] = "SequentialExecutor"
os.environ["_AIRFLOW__AS_LIBRARY"] = "1"
