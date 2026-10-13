"""Serve all example agents: ``python -m leforge_agents`` (PORT, HOST env)."""
import os

from leforge import serve

from . import all_agents


def main() -> None:
    serve(all_agents(), host=os.environ.get("HOST", "0.0.0.0"), port=int(os.environ.get("PORT", "8000")))


if __name__ == "__main__":
    main()
