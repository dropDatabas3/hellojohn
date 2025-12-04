import { Suspense } from "react"
import ScopesClientPage from "../[id]/scopes/ScopesClientPage"

export default function Page() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <ScopesClientPage />
    </Suspense>
  )
}
