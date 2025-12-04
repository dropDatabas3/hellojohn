import { Suspense } from "react"
import TenantDetailClientPage from "../[id]/TenantDetailClientPage"

export default function Page() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <TenantDetailClientPage />
    </Suspense>
  )
}
