-- Development-default role catalogue and grant seeds (DEC-004 pending owner
-- ratification). Explicit rows only — no wildcard grants (schema constraint).
-- Every allow at runtime requires one of these rows plus an active membership
-- (or exactly-granted support lease) on the owning scope.
-- Role IDs are kind-qualified (constraint: role has explicit workspace kind).

-- Roles per workspace kind: (role_id, workspace_kind, revision, title)
INSERT INTO role(role_id, workspace_kind, revision, title) VALUES
 ('server.owner','server',1,'Owner'),
 ('server.administrator','server',1,'Administrator'),
 ('server.operator','server',1,'Operator'),
 ('server.viewer','server',1,'Viewer'),
 ('hosting.owner','hosting_org',1,'Owner'),
 ('hosting.administrator','hosting_org',1,'Administrator'),
 ('hosting.reseller','hosting_org',1,'Reseller'),
 ('hosting.support_agent','hosting_org',1,'Support agent'),
 ('hosting.viewer','hosting_org',1,'Viewer'),
 ('website.owner','website',1,'Owner'),
 ('website.manager','website',1,'Manager'),
 ('website.developer','website',1,'Developer'),
 ('website.viewer','website',1,'Viewer'),
 ('developer.owner','developer_project',1,'Owner'),
 ('developer.maintainer','developer_project',1,'Maintainer'),
 ('developer.developer','developer_project',1,'Developer'),
 ('developer.viewer','developer_project',1,'Viewer');

-- Server workspace grants: (role_id, action_id, resource_type, scope_rule)
INSERT INTO grant_rule(role_id, action_id, resource_type, scope_rule) VALUES
 ('server.owner','host.inspect','host snapshot','same_scope'),
 ('server.owner','host.update','host update','same_scope'),
 ('server.owner','host.network.change','network plan','same_scope'),
 ('server.owner','host.place','placement','same_scope'),
 ('server.owner','storage.inspect','volume','same_scope'),
 ('server.owner','audit.read','audit event','same_scope'),
 ('server.owner','monitor.read','health observation','same_scope'),
 ('server.owner','security.policy.apply','scan or WAF policy','same_scope'),
 ('server.owner','module.install','module','same_scope'),
 ('server.owner','schedule.configure','schedule','same_scope'),
 ('server.owner','operation.retry','job','same_scope'),
 ('server.owner','runtime.toolchain.configure','runtime toolchain','same_scope'),
 ('server.administrator','host.inspect','host snapshot','same_scope'),
 ('server.administrator','host.update','host update','same_scope'),
 ('server.administrator','host.network.change','network plan','same_scope'),
 ('server.administrator','storage.inspect','volume','same_scope'),
 ('server.administrator','audit.read','audit event','same_scope'),
 ('server.administrator','monitor.read','health observation','same_scope'),
 ('server.administrator','module.install','module','same_scope'),
 ('server.administrator','schedule.configure','schedule','same_scope'),
 ('server.operator','host.inspect','host snapshot','same_scope'),
 ('server.operator','monitor.read','health observation','same_scope'),
 ('server.operator','schedule.configure','schedule','same_scope'),
 ('server.viewer','monitor.read','health observation','same_scope');

-- Hosting organization grants (provider/reseller/agency persona)
INSERT INTO grant_rule(role_id, action_id, resource_type, scope_rule) VALUES
 ('hosting.owner','account.provision','hosting account','same_scope'),
 ('hosting.owner','allocation.reserve','quota reservation','same_scope'),
 ('hosting.owner','billing.adjust','billing entry','same_scope'),
 ('hosting.owner','payment.record','payment event','same_scope'),
 ('hosting.owner','support.ticket.create','support ticket','same_scope'),
 ('hosting.owner','support.article.publish','knowledge article','same_scope'),
 ('hosting.owner','audit.read','audit event','same_scope'),
 ('hosting.owner','monitor.read','health observation','same_scope'),
 ('hosting.owner','membership.grant','membership','same_scope'),
 ('hosting.owner','template.install','app template','same_scope'),
 ('hosting.administrator','account.provision','hosting account','same_scope'),
 ('hosting.administrator','allocation.reserve','quota reservation','same_scope'),
 ('hosting.administrator','audit.read','audit event','same_scope'),
 ('hosting.administrator','monitor.read','health observation','same_scope'),
 ('hosting.administrator','membership.grant','membership','same_scope'),
 ('hosting.reseller','account.provision','hosting account','same_scope'),
 ('hosting.reseller','allocation.reserve','quota reservation','same_scope'),
 ('hosting.reseller','monitor.read','health observation','same_scope'),
 ('hosting.support_agent','support.ticket.create','support ticket','same_scope'),
 ('hosting.support_agent','monitor.read','health observation','same_scope'),
 ('hosting.viewer','monitor.read','health observation','same_scope');

-- Website workspace grants (website owner persona)
INSERT INTO grant_rule(role_id, action_id, resource_type, scope_rule) VALUES
 ('website.owner','file.write','filesystem entry','same_scope'),
 ('website.owner','database.provision','SQL database','same_scope'),
 ('website.owner','cache.configure','cache instance','same_scope'),
 ('website.owner','backup.restore','backup artifact','same_scope'),
 ('website.owner','domain.bind','hostname binding','same_scope'),
 ('website.owner','sftp.grant','SFTP credential','same_scope'),
 ('website.owner','vhost.apply','vhost','same_scope'),
 ('website.owner','schedule.configure','schedule','same_scope'),
 ('website.owner','template.install','app template','same_scope'),
 ('website.owner','mail.domain.configure','mail domain','same_scope'),
 ('website.owner','mailbox.provision','mailbox','same_scope'),
 ('website.owner','audit.read','audit event','same_scope'),
 ('website.owner','monitor.read','health observation','same_scope'),
 ('website.owner','site.log.read','site log view','same_scope'),
 ('website.owner','terminal.open','terminal lease','same_scope'),
 ('website.owner','zone.publish','DNS zone','same_scope'),
 ('website.manager','file.write','filesystem entry','same_scope'),
 ('website.manager','schedule.configure','schedule','same_scope'),
 ('website.manager','monitor.read','health observation','same_scope'),
 ('website.manager','site.log.read','site log view','same_scope'),
 ('website.developer','file.write','filesystem entry','same_scope'),
 ('website.developer','monitor.read','health observation','same_scope'),
 ('website.viewer','monitor.read','health observation','same_scope');

-- Developer project grants (developer persona)
INSERT INTO grant_rule(role_id, action_id, resource_type, scope_rule) VALUES
 ('developer.owner','source.connect','source connection','same_scope'),
 ('developer.owner','deployment.promote','release artifact','same_scope'),
 ('developer.owner','release.certify','release candidate','same_scope'),
 ('developer.owner','environment.clone','environment','same_scope'),
 ('developer.owner','runtime.configure','runtime toolchain','same_scope'),
 ('developer.owner','integration.configure','integration grant','same_scope'),
 ('developer.owner','terminal.open','terminal lease','same_scope'),
 ('developer.owner','audit.read','audit event','same_scope'),
 ('developer.owner','monitor.read','health observation','same_scope'),
 ('developer.owner','application.provision','application','same_scope'),
 ('developer.maintainer','source.connect','source connection','same_scope'),
 ('developer.maintainer','deployment.promote','release artifact','same_scope'),
 ('developer.maintainer','environment.clone','environment','same_scope'),
 ('developer.maintainer','monitor.read','health observation','same_scope'),
 ('developer.developer','source.connect','source connection','same_scope'),
 ('developer.developer','monitor.read','health observation','same_scope'),
 ('developer.viewer','monitor.read','health observation','same_scope');
