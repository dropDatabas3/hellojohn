import { Suspense } from "react";
import FormsClientPage from "./FormsClientPage";

export default function Page() {
    return (
        <Suspense fallback={<div>Loading...</div>}>
            <FormsClientPage />
        </Suspense>
    );
}
