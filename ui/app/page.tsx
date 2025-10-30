// Root page - redirect to /home

import { redirect } from "next/navigation"

export default function RootRedirect() {
  redirect("/home")
}
