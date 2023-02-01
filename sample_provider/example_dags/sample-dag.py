from pendulum import datetime

from airflow.decorators import dag

from sample_provider.operators.sample_operator import SampleOperator
from sample_provider.sensors.sample_sensor import SampleSensor


@dag(
    start_date=datetime(2022, 1, 1),
    schedule_interval=None,
    tags=["example"],
)
def sample_workflow():
    """
    ### Sample DAG

    Showcases the sample provider package's operator and sensor.

    To run this example, create an HTTP connection with:
    - id: conn_sample
    - type: http
    - host: www.httpbin.org
    """

    task_get_op = SampleOperator(task_id="get_op", method="get")

    task_sensor = SampleSensor(task_id="sensor", sample_conn_id="conn_sample", endpoint="")

    task_get_op >> task_sensor


sample_workflow_dag = sample_workflow()
