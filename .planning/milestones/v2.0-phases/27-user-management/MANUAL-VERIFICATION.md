# Phase 27: User Management — Manual Verification Checklist

## Setup

1. Start dev servers: `make run-server` and `cd web && pnpm dev`
2. Open http://localhost:5173/login

## Admin User Checks

- [ ] Log in as admin
- [ ] "Users" appears in sidebar nav
- [ ] Navigate to `/users` — table shows Username, Role (badge), Status (badge), Created columns
- [ ] Your own row does NOT show the three-dot action menu

## Create User

- [ ] Click "Create User" button
- [ ] Fill form: username, password (8+ chars), select role, display name, email
- [ ] Submit — toast "User created." appears
- [ ] New user shows in table

## Deactivate User

- [ ] Click three-dot menu on new user → "Deactivate"
- [ ] Confirmation dialog appears with red "Deactivate" button
- [ ] Confirm — toast appears, status badge changes to "Inactive"

## Reactivate User

- [ ] Click three-dot menu on inactive user → "Reactivate"
- [ ] Toast appears, status returns to "Active"

## Reset Password

- [ ] Click three-dot menu → "Reset Password"
- [ ] Enter new password (8+ chars) with confirmation — submit
- [ ] Toast "Password reset for {username}." appears

## Cross-User Verification

- [ ] Log out, log in as the password-reset user with new password — login works
- [ ] Log in as non-admin user — "Users" does NOT appear in sidebar
