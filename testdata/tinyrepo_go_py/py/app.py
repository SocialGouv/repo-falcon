from .util import double
from ..py.models import User
import json


def handler(x: int) -> str:
    return json.dumps({"value": double(x)})


class AppConfig:
    debug = False
