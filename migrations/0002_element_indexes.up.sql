-- Elements are commonly filtered by cluster and by assignment status
-- (the Elements UI page's two main filters), and listed sorted by
-- last_seen_at; index all three to keep those queries cheap as the
-- table grows past a trivial row count.
CREATE INDEX idx_network_elements_cluster_id ON network_elements (cluster_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_network_elements_assignment_status ON network_elements (assignment_status) WHERE deleted_at IS NULL;
CREATE INDEX idx_network_elements_last_seen_at ON network_elements (last_seen_at DESC) WHERE deleted_at IS NULL;
