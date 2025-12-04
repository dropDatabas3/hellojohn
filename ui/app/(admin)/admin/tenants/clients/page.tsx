import { Suspense } from "react"
import ClientsClientPage from "../[id]/clients/ClientsClientPage"

export default function Page() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <ClientsClientPage />
    </Suspense>
  )
}
