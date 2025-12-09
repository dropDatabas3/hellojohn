
import * as React from "react"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { COUNTRIES } from "@/lib/countries"

interface CountrySelectProps {
    value?: string
    onChange: (value: string) => void
    disabled?: boolean
}

export function CountrySelect({ value, onChange, disabled }: CountrySelectProps) {
    // Sort countries alphabetically
    const sortedCountries = React.useMemo(() => {
        return [...COUNTRIES].sort((a, b) => a.name.localeCompare(b.name))
    }, [])

    // Find selected country to display custom trigger if needed (optional with SelectValue)
    // Radix SelectValue automatically displays the content of the selected SelectItem.

    return (
        <Select value={value} onValueChange={onChange} disabled={disabled}>
            <SelectTrigger className="w-full">
                <SelectValue placeholder="Seleccionar país..." />
            </SelectTrigger>
            <SelectContent className="max-h-[300px]">
                {sortedCountries.map((country) => (
                    <SelectItem key={country.code} value={country.name}>
                        <div className="flex items-center gap-2">
                            <img
                                src={`https://flagcdn.com/w20/${country.code.toLowerCase()}.png`}
                                srcSet={`https://flagcdn.com/w40/${country.code.toLowerCase()}.png 2x`}
                                width="20"
                                height="15"
                                alt={country.name}
                                className="object-contain"
                            />
                            <span>{country.name}</span>
                        </div>
                    </SelectItem>
                ))}
            </SelectContent>
        </Select>
    )
}
