FROM ghcr.io/astral-sh/uv:python3.12-bookworm-slim

# install uv 
# https://docs.astral.sh/uv/guides/integration/docker/#getting-started
COPY --from=ghcr.io/astral-sh/uv:latest /uv /uvx /usr/local/bin/

# working directory
WORKDIR /app

# install system dependencies by uv
RUN uv pip install --system fastmcp requests uvicorn

COPY tb-mcp.py .

RUN mkdir -p /app/log

EXPOSE 8000

ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1
ENV MCP_SERVER_HOST=0.0.0.0
ENV MCP_SERVER_PORT=8000

# Run the MCP server python by fastmcp
# SSE transport is used
CMD ["uv", "run", "./tb-mcp.py"]