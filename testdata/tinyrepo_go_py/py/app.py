from .util import double
import json


def handler(x: int) -> str:
    return json.dumps({"value": double(x)})

