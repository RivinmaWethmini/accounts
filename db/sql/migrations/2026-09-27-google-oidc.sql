-- Add Google as an OAuth/OIDC connection provider
INSERT INTO Connections (provider) VALUES ('google') ON CONFLICT DO NOTHING;
