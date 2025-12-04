import { Suspense } from "react"
import TokensClientPage from "../[id]/tokens/TokensClientPage"

export default function Page() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <TokensClientPage />
    </Suspense>
  )
}
