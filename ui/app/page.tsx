// Root page - redirect unauthenticated users to /login

import { redirect } from "next/navigation"

export default function RootRedirect() {
  // Simple default: send users to the login page.
  // After successful login, the app will route them to /admin.
  redirect("/login")
}
