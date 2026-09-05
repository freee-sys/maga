CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE clusters (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    lookup_key    text NOT NULL UNIQUE,
    display_name  text NOT NULL,
    description   text,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE rules (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name                  text NOT NULL,
    description           text,
    pattern               text NOT NULL,
    capture_transforms    jsonb NOT NULL DEFAULT '{}',
    cluster_key_template  text NOT NULL,
    priority              integer NOT NULL,
    is_enabled            boolean NOT NULL DEFAULT true,
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    UNIQUE (priority)
);

CREATE TABLE network_elements (
    id                 uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    ip_address         inet NOT NULL,
    sys_name           text,
    sys_descr          text,
    sys_object_id      text,
    sys_uptime_ticks   bigint,
    attributes         jsonb NOT NULL DEFAULT '{}',
    cluster_id         uuid REFERENCES clusters(id),
    assignment_status  text NOT NULL DEFAULT 'unassigned'
                        CHECK (assignment_status IN ('unassigned','assigned')),
    last_rule_id       uuid REFERENCES rules(id),
    first_seen_at      timestamptz NOT NULL DEFAULT now(),
    last_seen_at       timestamptz NOT NULL DEFAULT now(),
    deleted_at         timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    UNIQUE (ip_address)
);

CREATE TABLE discovery_jobs (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_type        text NOT NULL DEFAULT 'snmp_discovery',
    input_spec      text NOT NULL,
    target_ips      inet[] NOT NULL,
    snmp_version    text NOT NULL DEFAULT 'v2c' CHECK (snmp_version IN ('v1','v2c')),
    snmp_community  text NOT NULL,
    status          text NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending','running','completed','failed','cancelled')),
    total_targets   integer NOT NULL,
    started_at      timestamptz,
    completed_at    timestamptz,
    error_message   text,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE discovery_job_results (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id         uuid NOT NULL REFERENCES discovery_jobs(id) ON DELETE CASCADE,
    ip_address     inet NOT NULL,
    status         text NOT NULL CHECK (status IN ('success','no_response','snmp_error')),
    sys_name       text,
    sys_descr      text,
    sys_object_id  text,
    error_message  text,
    element_id     uuid REFERENCES network_elements(id),
    responded_at   timestamptz,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_job_results_job_status ON discovery_job_results (job_id, status);

CREATE TABLE assignment_history (
    id                   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    element_id           uuid NOT NULL REFERENCES network_elements(id) ON DELETE CASCADE,
    cluster_id           uuid REFERENCES clusters(id),
    rule_id              uuid REFERENCES rules(id),
    matched_pattern      text,
    raw_captures         jsonb NOT NULL DEFAULT '{}',
    transformed_captures jsonb NOT NULL DEFAULT '{}',
    rendered_cluster_key text,
    trigger_source       text NOT NULL
                          CHECK (trigger_source IN
                            ('discovery','manual_redistribute_single','manual_redistribute_bulk')),
    warnings             text,
    created_at           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_assignment_history_element ON assignment_history (element_id, created_at DESC);
