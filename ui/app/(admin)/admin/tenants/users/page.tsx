import { Suspense } from "react"
import UsersClientPage from "../[id]/users/UsersClientPage"

export default function Page() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <UsersClientPage />
    </Suspense>
  )
}
