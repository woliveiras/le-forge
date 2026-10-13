# LeForge Evolution Plan

**Goal:** Make the web interface the place where users configure models and integrations, build graphs, and monitor agents at work. LangGraph remains supported; reusable components are deferred to the second release.

## Product Principles

- **The web interface is the primary path:** Normal workflows must not require server access or CLI usage.
- **Configurable models:** Use the provider and credentials configured by the user, without coupling the platform to a vendor.
- **Bounded autonomy:** Agents may delegate work, but they cannot expand their own permissions or consume unlimited resources.
- **Security independent of instructions:** Skills and steering guide agents but do not replace platform-enforced permissions.
- **Genuine visual construction:** The canvas must allow users to edit graphs, not just view those already defined.
- **Users operate; they do not edit files:** Users send commands/instructions through the interface; only agents may modify workspace files. Users may inspect files and diffs.

## Release 1 — Build and Operate Graphs Through the Interface

### 1. Remove Out-of-Scope Product Functionality

Remove compatibility features and references that are not part of the planned product:

- converter and import endpoint;
- conversion button, examples, and related tests;
- remaining documentation, references, and dependencies for that compatibility.

**Acceptance criterion:** The repository and application contain no compatibility flows, references, or documentation for out-of-scope external products.

### 2. Complete Model Configuration

Preserve support for LangGraph and the existing model provider system. Make provider configuration available in the interface so users can:

- add and edit providers;
- store credentials securely without displaying them again after saving;
- choose the provider and model for each agent;
- validate the connection and show actionable errors.

**Acceptance criterion:** Users configure credentials through the interface and run an agent using the selected provider, without configuring secrets directly on the server during normal use.

### 3. Turn the Canvas into a Graph Editor

Evolve the existing `GraphCanvas.tsx` view to allow users to:

- create, move, configure, and remove agents;
- create, edit, and remove channels connecting agents;
- configure agent and channel properties in panels;
- validate the graph before saving;
- identify unsaved changes, errors, and version conflicts;
- keep the YAML editor as an advanced option.

**Acceptance criterion:** Users can create, configure, save, and start a complete graph in the browser without editing YAML or accessing the machine running the service.

### 4. Create the File and Interaction Panel

Add a per-graph workspace to the console, including:

- a file tree and content viewer;
- Git diff, branch, directory, and responsible replica views;
- links between files, messages, and executions;
- an interface for sending tasks/commands to agents and tracking progress;
- a history of agent-made changes, with review and approval according to the configured policy.

Users may read and inspect files, but cannot edit, delete, or upload them through the console. Only authorized agents may modify the workspace; every operation must respect workspace boundaries and permissions without exposing the server's filesystem.

**Acceptance criterion:** Users can start and direct tasks, monitor agents, and inspect changes entirely through the browser, without server access or CLI usage.

### 5. Add MCP Configuration

Implement MCP connection management in the interface, starting with the integrations used by the team:

- Bitbucket;
- GitHub;
- Confluence;
- Jira.

Users must be able to add multiple connections for each service, edit, test, disable, and remove connections; inspect available tools; and grant access to specific agents by connection/tool.

Use connections and transports that can be reached securely from the Docker environment. Store tokens and credentials encrypted, hide them after saving, and allow revocation/rotation. Calls go through the LeForge gateway, with per-agent authorization, call/error auditing, limits, and approvals when configured. Do not execute arbitrary user commands in the main process to start local MCP servers.

**Acceptance criterion:** Users can connect the four priority integrations through the interface, test connections, grant tools to specific agents, and inspect calls and their results.

### 6. Add Skills and Steering

Use hybrid storage with separate responsibilities:

- **Application:** Installation-wide instructions, user-managed skills, and precedence/permission configuration.
- **User repository:** Project-specific steering and instructions, versioned with the code and loaded only for graphs authorized to use that workspace.

Define clear precedence: global, graph, agent, and task instructions; repository instructions belong to the project scope and cannot override security policies. Show the source and effective version/content of instructions loaded in each execution. Treat repository content as untrusted input: it may guide the agent, but must never grant tools, credentials, or permissions.

**Acceptance criterion:** Operators can manage global instructions through the application; project-specific instructions travel with the repository; every execution records the loaded instruction set; no instruction can elevate privileges.

### 7. Allow Dynamic Delegation to Subagents

Give orchestrator agents the ability to delegate subtasks when they determine it is necessary, without requiring users to pre-draw every subagent in the graph.

Start with **temporary child executions**, not unrestricted edits to the permanent graph. Each delegation records its parent, objective, agent/configuration used, result, state, and resource consumption.

Allow operators to configure the delegation policy:

- **Approval required:** Each new delegation waits for human approval before starting.
- **Automatic:** Delegations start without individual approval, subject to configured global limits.

Set global limits for depth, number of children, concurrency, duration, and cost. Child permissions must be the same as or more restrictive than the delegating agent's permissions. Configuration and approval policy are controlled by the operator, never by the agent.

**Acceptance criterion:** An agent can delegate, receive the result, and continue; the interface displays the execution tree and can stop executions; approval and limits are enforced by the server.

### 8. Prepare Local Docker Operation

Review deployment and documentation so the primary workflow is:

1. start the application on local infrastructure;
2. open the console in a browser;
3. configure providers, integrations, and permissions;
4. create or open a graph;
5. send tasks/commands to agents and monitor executions, approvals, and changes through the interface.

**Acceptance criterion:** Setting up and operating a new installation does not depend on editing files on the server or submitting tasks through the CLI.

### 9. Guide First Use with Onboarding

Add an onboarding experience in the interface to introduce LeForge and guide operators from initial configuration through their first completed task. The journey must be progressive, resumable, and dismissible without blocking experienced users; guidance must remain available after completion.

The onboarding should guide users to:

- understand the basics of graphs, agents, channels, and tools;
- check installation services and configure/test a model provider;
- discover and configure MCP connections and permissions when available;
- create a first graph on the canvas, configure agents, and connect them with channels;
- start a task and monitor agents, messages, executions, and approvals;
- inspect resulting files and diffs while respecting the user's read-only access.

Include a safe, working example graph to explore or duplicate, clearly indicating which settings must be adjusted before running it. Show unconfigured dependencies and provide paths to the relevant screens; do not claim success until real tests pass. Let users return to tutorials and contextual help without restarting or losing existing work.

**Acceptance criterion:** A new operator can follow onboarding in the browser, understand the essential concepts, configure available dependencies, create or adapt an example graph, and run their first task. They can skip steps, resume the journey, and revisit help without server or CLI access.

## Release 2 — Reusable Components

After graph creation and operation have stabilized:

- allow users to save agents and subgraphs as components;
- insert components into other graphs through the visual editor;
- define editable parameters when inserting a component;
- version components and indicate available updates;
- allow sharing according to installation permissions.

**Acceptance criterion:** Users can reuse a component across graphs without manually copying definitions or breaking existing configurations.

## Suggested Implementation Sequence

1. **Scope cleanup** and confirmation of the flows that remain.
2. **API contracts and data model** for the visual editor, workspace, providers, MCP integrations, instructions, and child executions.
3. **Visual editor**, as the central experience and a dependency for the graph model.
4. **File and interaction panel**, read-only for users, with tasks and changes tracked by execution.
5. **Web configuration for models and MCPs**, with protected credentials and Bitbucket, GitHub, Confluence, and Jira integrations.
6. **Skills and steering**, with hybrid storage, precedence, traceability, and trust boundaries.
7. **Dynamic delegation**, with a configurable choice between individual approval and bounded automatic execution.
8. **Guided onboarding**, integrated into delivered screens and flows, with an example graph and resumable help.
9. **End-to-end tests** for the full interface workflow, including onboarding, authorization, and isolation.
10. **Component library** as the second release.
