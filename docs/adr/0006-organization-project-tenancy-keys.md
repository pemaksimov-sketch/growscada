# Every operational entity is keyed by Project, every Project by Organization

A server hosts many Projects in both node roles, and each Project belongs to one Organization (the tenant). Tags, Scenes with their Widgets, Devices, the Draft, Revisions, the Journal and Checkpoints all carry `project_id`, natural keys are scoped to the Project (a Tag is identified by Project + name, see ADR 0003), and Projects never reference each other's entities. We introduce these keys before the Engineering/Runtime storage split, Revisions and the Journal exist, because adding them afterwards would be a second migration of every table, every journaled payload and every Checkpoint. Until Users exist (task 49) the server runs with one seeded Organization; the key is there, the multi-tenant UX comes later.

## Implemented scope

Task 31 uses a typed repository factory, `ForProject(ProjectID).Tags()/Scenes()`.
No request-context key supplies isolation. Every read, CAS update and delete is
scoped, widgets carry their scene's Project through a composite foreign key,
and both outboxes retain `project_id` after an owner is deleted. SSE filters
the committed scope before enqueueing a client's message. The shared Library
remains organization-wide; until task 33 replaces live reconciliation with
release pinning, its policy enumerates Projects and obtains each one's scoped
repositories. API 0.3 adds Project CRUD and explicit project resource routes.
Unscoped resource paths remain aliases for Main to read older clients safely.
The one seeded Organization has no authentication or permissions yet (49–50).

## Considered Options

- One Runtime node = one Project, isolation by deployment: rejected. A plant with several lines, or an integrator hosting several customers, would need a server per Project.
- Many Projects in the Engineering node only, one Project per Runtime node: rejected for the same reason on the runtime side.
- One shared pool of Tags per server with Projects as different views over it: rejected. Two Projects built from one template would collide on Tag names.
