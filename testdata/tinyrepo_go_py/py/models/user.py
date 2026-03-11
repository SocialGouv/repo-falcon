from dataclasses import dataclass


@dataclass
class User:
    name: str
    age: int

    def greet(self) -> str:
        return f"Hello, {self.name}"


def create_user(name: str, age: int) -> User:
    return User(name=name, age=age)
