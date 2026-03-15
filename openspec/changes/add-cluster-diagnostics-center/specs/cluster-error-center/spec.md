## ADDED Requirements

### Requirement: Agent SHALL collect Seatunnel ERROR events from engine and job logs
The system SHALL use the managed Agent to incrementally scan Seatunnel log files under each tracked install directory and extract ERROR events from both engine logs and routed job logs. The collection pipeline MUST support `seatunnel-engine-*.log` and `job-*.log`, and MUST continue to work when log4j2 `routingAppender` routes job errors away from the worker or master main log.

#### Scenario: Collect error from engine log
- **WHEN** a managed Seatunnel node writes an `ERROR` entry into `seatunnel-engine-worker.log`, `seatunnel-engine-master.log`, or `seatunnel-engine-server.log`
- **THEN** the Agent SHALL extract the error event and report it to the control plane with cluster, node, role, install directory, source file, timestamp, and message context

#### Scenario: Collect error from routed job log
- **WHEN** a managed Seatunnel job writes an `ERROR` entry into a routed `job-*.log` file
- **THEN** the Agent SHALL extract the error event from that job log instead of requiring the error to appear in the main engine log

#### Scenario: Continue collection when auto restart is disabled
- **WHEN** a managed cluster keeps auto monitoring enabled but disables auto restart
- **THEN** the Agent SHALL continue to maintain diagnostics scan targets and collect Seatunnel ERROR events for the managed install directories

#### Scenario: Continue collection across log rotation
- **WHEN** a Seatunnel active log file is rotated to a dated suffix file such as `.log.2026-03-10-1` before the next diagnostics scan
- **THEN** the Agent SHALL recover the unread tail from the latest non-compressed rotated file and continue collecting from the new active file without requiring manual reset

### Requirement: The control plane SHALL persist structured error evidence without storing full raw logs
The system SHALL persist Seatunnel errors as structured diagnostic evidence. At minimum, it MUST maintain individual error events, aggregated error groups, and file read cursors for incremental collection. The stored evidence MUST include enough context to support triage, grouping, and later AI analysis without requiring full raw log retention.

#### Scenario: Persist a new error event
- **WHEN** the control plane receives a Seatunnel error report that does not match an existing cursor position
- **THEN** it SHALL persist an error event record, update the corresponding cursor, and attach the event to an error group based on the normalized fingerprint

#### Scenario: Repeated error is grouped
- **WHEN** multiple ERROR events share the same normalized fingerprint within the grouping window
- **THEN** the control plane SHALL increment the occurrence count and update first-seen / last-seen metadata on the same error group instead of creating duplicate groups

#### Scenario: Wrapper messages are grouped by shared root cause
- **WHEN** one Seatunnel failure produces wrapper messages such as `Failed to initialize connection...` and outer `submit job ... SeaTunnelRuntimeException...` records that share the same normalized root cause
- **THEN** the control plane SHALL normalize them into the same error fingerprint group instead of splitting them only because outer wrapper text differs

### Requirement: The system SHALL provide global and cluster-scoped error center views
The system SHALL provide a diagnostic error center UI and APIs that support global browsing and cluster-filtered browsing of Seatunnel error groups and events. Users MUST be able to filter by cluster, node, host, role, job identifier, time range, and error keyword or exception class.

#### Scenario: Open global error center
- **WHEN** a user opens the diagnostics workspace without a cluster filter
- **THEN** the system SHALL show cross-cluster error groups sorted by recent activity and impact

#### Scenario: Open cluster-scoped error center
- **WHEN** a user enters the error center from a cluster detail page or with a cluster filter in the URL
- **THEN** the system SHALL pre-apply the cluster scope and show only errors related to that cluster

#### Scenario: Reveal full source file path from truncated UI text
- **WHEN** the diagnostics error center truncates a long source file path in the events table
- **THEN** the UI SHALL reveal the full path on hover so operators can verify the exact Seatunnel log file being referenced

### Requirement: The system SHALL support diagnostic navigation from related surfaces
The system SHALL support navigation into the error center from cluster detail, alert center, and future diagnostic task or inspection results, preserving the relevant context in the target page.

#### Scenario: Navigate from cluster detail to error center
- **WHEN** a user clicks the diagnostics or recent error shortcut on a cluster detail page
- **THEN** the system SHALL open the diagnostics workspace with the cluster filter preselected

#### Scenario: Navigate from alert to related errors
- **WHEN** a user views an alert that is associated with a cluster, node, or error fingerprint
- **THEN** the system SHALL offer a link into the error center with matching filters applied
