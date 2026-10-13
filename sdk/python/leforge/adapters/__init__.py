"""Framework adapters.

Each adapter wraps a third-party agent framework so its agents run as
LeForge nodes. The framework is imported lazily: importing this package
never requires LangGraph, AutoGen or CrewAI to be installed, and using an
adapter without its framework raises :class:`FrameworkNotInstalled` with
the pip extra to install.

In every adapter the framework's model calls and tool calls are routed
through the TurnContext - i.e. through the platform's LLM gateway and tool
gateway - so grants, approvals, auditing and tracing apply unchanged.
"""
from __future__ import annotations

import importlib
from typing import Any


class FrameworkNotInstalled(ImportError):
    pass


def require(module: str, extra: str) -> Any:
    try:
        return importlib.import_module(module)
    except ImportError as e:
        raise FrameworkNotInstalled(
            f"{module} is not installed; install it with `pip install leforge[{extra}]`"
        ) from e


def available(module: str) -> bool:
    try:
        importlib.import_module(module)
        return True
    except ImportError:
        return False


from .common import gateway_tool_functions  # noqa: E402
from .langgraph import LangGraphAgent, leforge_chat_model, leforge_tools_for_langchain  # noqa: E402
from .autogen import AutoGenAgent, leforge_model_client, leforge_autogen_tools  # noqa: E402
from .crewai import CrewAIAgent, leforge_crewai_llm, leforge_crewai_tools  # noqa: E402

__all__ = [
    "FrameworkNotInstalled", "require", "available", "gateway_tool_functions",
    "LangGraphAgent", "leforge_chat_model", "leforge_tools_for_langchain",
    "AutoGenAgent", "leforge_model_client", "leforge_autogen_tools",
    "CrewAIAgent", "leforge_crewai_llm", "leforge_crewai_tools",
]
