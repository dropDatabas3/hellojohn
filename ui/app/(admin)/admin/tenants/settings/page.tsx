import { Suspense } from "react"
import SettingsClientPage from "../[id]/settings/SettingsClientPage"

export default function Page() {
  return (
    <Suspense fallback={<div>Loading...</div>}>
      <SettingsClientPage />
    </Suspense>
  )
}
