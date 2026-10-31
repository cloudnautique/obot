# 2026-10-30: Admin Read-Only as a base role

- **Status:** Accepted
- **Date:** 2026-10-30
- **Supersedes:** None
- **Superseded by:** None

## Related ODPs

None. There is no ODP for this decision.

## Context

Customers want to let someone see Obot's configuration without being able to change it. Roles are bit flags (`apiclient/types/user.go`): one base role (Owner, Admin, Power User Plus, Power User or Basic) plus the orthogonal Auditor flag. A user's effective role is their own role ORed with the roles of their group role assignments, then normalized to the highest base role plus Auditor if any source had it (`normalizeToHighestRole` in `pkg/gateway/client/grouproleassignment.go`). Before this decision, a user therefore held exactly one base role.

The only way to get read-only access to the admin area was the Auditor flag on a non-admin base role. Auditor also grants sensitive data: MCP, LLM and egress audit log request and response payloads, auth provider reveal, and audit log exports with their storage credentials. Granting someone read-only access to configuration meant also granting them that data, and assigning Auditor is reserved to owners.

## Decision

Add **Admin Read-Only** (`RoleAdminReadonly`, group `admin-readonly`) as a new base role.

- **Ranking.** Owner > Admin > Admin Read-Only > Power User Plus > Power User > Basic. Normalization keeps only the highest, with one exception: Admin Read-Only keeps the highest of Power User Plus and Power User beside it. Owner and Admin still replace lower roles, which they already include.
- **Admin Read-Only adds to a power user's role.** However a power user is granted Admin Read-Only, they keep their power user role beside it, so they keep their workspace and what they published in it, and gain read access to the admin area:
  - Through a group role assignment, normalization keeps the power user role when the user's own role and their groups' roles are merged (`normalizeToHighestRole`).
  - Assigned directly, the server keeps the power user role the user already had (`Role.KeepPowerUserFrom` in `updateUser`). A user's stored role can therefore be Admin Read-Only combined with Power User Plus or Power User. An admin demoted directly to Admin Read-Only keeps Power User Plus, which Admin includes, and so keeps their workspace too.
  - To take the power user role away, an admin changes the user to a role without Admin Read-Only, such as Standard User. That removes the workspace, as any change away from Power User does.
  - The Update Role dialog shows a combined role as Admin Read-Only; the role label names both, such as "Admin Read-Only, Power User Plus".
- **Access.** Admin Read-Only can read whatever an admin can read and can change nothing. Its static authorization rules (`adminReadonlyRules` in `pkg/api/authz/authz.go`) are GET routes only. It is excluded from:
  - every `reveal` endpoint;
  - action routes that are reads in name only (tool previews, the MCP tester, guardrail and storage credential tests);
  - MDM installer downloads, which can enroll devices;
  - setup and debug endpoints;
  - the unmasked license key;
  - a pending auth provider switch, which is shown only to admins, who can act on it.

  Audit logs appear as they do for an admin: without request and response payloads.
- **Auditor is unchanged.** Its rules and meaning stay the same, and it can be combined with Admin Read-Only (`RoleAdminReadonly | RoleAuditor`) to add payloads and exports.
- **Two "can view" checks.** `Role.groups()` adds the `admin-readonly` group for Owner, Admin and Admin Read-Only, but not for Auditor, whose access stays exactly as it was.
  - Handlers and authorizers that granted privileged reads to "admin or auditor" now use one check that also admits Admin Read-Only: `UserCanViewAdmin()` on the request and `User.CanViewAdmin()` in the authorizer.
  - Handlers that showed everything only to admins, such as the list of all workspaces or all Claude sessions, now use `UserCanReadAsAdmin()`. It admits owners, admins and Admin Read-Only but not auditors.
  - In the UI these are `hasAdminAccess()` and `isAdminReadonly()`.
  - Write paths, audit log payloads, the license key, and filters that hide what only an admin could set up (catalog entries whose OAuth credentials are not configured) keep their admin or auditor checks.
- **Assignment.** Admins assign and remove Admin Read-Only like Admin: directly on a user or through a group role assignment. Unlike Auditor, it does not require an owner.
- **Not a default role.** The default role for new users remains Basic. The server rejects Admin Read-Only as the default role, not only the UI.
- **Role validation.** Changing a user's role now rejects values that are not one known base role, or Admin Read-Only with one of Power User Plus and Power User, plus, optionally, Auditor; previously any integer was accepted. A role that is not changing is not checked, so users whose stored role predates the check can still update other fields. Group role assignments already validated their base role and now accept Admin Read-Only. The default role, previously unchecked, must be Basic, Power User, Power User Plus or Admin: the roles the UI offers.

## Rationale

- **A base role rather than an add-on flag.** A flag would let Admin Read-Only be assigned beside any role, so whether a user can change admin configuration would depend on the combination. A base role keeps the answer simple to assign and audit, matching what customers ask for.
- **Keeping Power User beside Admin Read-Only, from groups and directly.** Replacing a power user's role with Admin Read-Only would drop Power User, and the workspace reconcile deletes the workspace, and the MCP servers, catalog entries and access rules published in it, of a user who loses Power User. Any admin could then destroy a power user's work just by granting them read-only access, whether through a group or directly. Keeping the power user role does not let anyone change admin configuration; it only leaves the user what they already had. Keeping it on direct assignment too means the same grant gives the same result whichever way it is made.
- **A new role rather than narrowing Auditor.** Narrowing Auditor to sensitive data only, and migrating existing Auditor users to Admin Read-Only plus Auditor, would make the model cleaner. However, it would be a breaking migration, and users with Power User or Power User Plus plus Auditor would lose their base role. Leaving Auditor alone means no existing user's access changes.
- **An explicit read-only rule list rather than deriving one from the admin rules.** Many admin rules are method-less prefixes, some reads are POSTs (reveals), and some GETs are sensitive. An explicit list, checked by a test against the admin rules, makes every exposure a deliberate choice.
- **Auditors keep their own rules rather than joining the new group.** The Auditor rules leave out some admin reads on purpose (for example MCP tunnels and the secrets that may be bound), and existing tests pin that. Giving auditors the new group would have widened their access without anyone choosing to.

## Consequences

- Every new admin GET route must be either added to `adminReadonlyRules` or listed as an intentional exclusion in `adminReadonlyExcludedReads`; `TestAdminReadonlyRulesCoverAdminReads` fails otherwise. `TestAdminReadonlyCannotMutate` fails if the role can make any change its baseline role can't.
- Admin pages must honour read-only mode for every mutating control. Pages that relied on only admins reaching them now need read-only handling, which also fixes the same gaps Auditors had.
- Admin Read-Only with Auditor can read slightly more than Basic with Auditor: the union of both rule lists.
- A stored or effective role can be Admin Read-Only combined with Power User Plus or Power User. Code that reads a role must use `HasRole`, not compare it with one role.
- An admin cannot move a power user straight to Admin Read-Only alone; they change the user to Standard User first, then grant Admin Read-Only.
- `TestAdminReadonlyRulesCoverAdminReads` covers only static authorization rules. A GET handler that filters what a non-admin sees by `UserIsAdmin()` must be checked by hand when widening what Admin Read-Only reads.
- A new or reranked base role must be checked against the workspace reconcile, since normalization drops lower base roles.
- Role bit 256 is now used. Owner and Admin users get an extra group, `admin-readonly`, in their identity.
- A read-only admin can still start an OAuth flow for their own connection to a server through the `oauth-url` reads, as any user with access to it can. That changes only their own credentials, not configuration.

## References

- `apiclient/types/user.go`: roles, groups and role hierarchy
- `pkg/api/authz/authz.go`: static rules for admin, Admin Read-Only and Auditor
- `pkg/api/authz/adminreadonly_test.go`: drift and no-mutation tests
- `pkg/api/authz/user.go`, `pkg/api/authz/ui.go`: privileged read checks
- `pkg/api/request.go`: `UserCanViewAdmin()`
- `pkg/gateway/client/grouproleassignment.go`: effective role resolution and normalization
- `pkg/controller/handlers/poweruserworkspace/`: workspace reconcile on role changes
- `pkg/gateway/server/user.go`: role assignment rules
- `ui/user/src/lib/services/user/operations.ts`: profile role helpers
- `ui/user/src/lib/services/admin/constants.ts`, `ui/user/src/routes/identity-access/`: role pickers
