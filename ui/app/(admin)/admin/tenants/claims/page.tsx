import { Suspense } from "react"
import ClaimsClientPage from "../[id]/claims/ClaimsClientPage"

export default function Page() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <ClaimsClientPage />
    </Suspense>
  )
}
