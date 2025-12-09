
import * as React from "react"
import { Input } from "@/components/ui/input"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { COUNTRIES, Country } from "@/lib/countries"

interface PhoneInputProps {
    value?: string
    onChange: (value: string) => void
    disabled?: boolean
}

export function PhoneInput({ value, onChange, disabled }: PhoneInputProps) {
    const [phoneNumber, setPhoneNumber] = React.useState("")
    // Default to Argentina if no match
    const defaultCountry = COUNTRIES.find((c) => c.code === "AR") || COUNTRIES[0]
    const [selectedCountryCode, setSelectedCountryCode] = React.useState<string>(defaultCountry.code)

    const sortedCountries = React.useMemo(() => {
        return [...COUNTRIES].sort((a, b) => a.name.localeCompare(b.name))
    }, [])

    const selectedCountry = COUNTRIES.find(c => c.code === selectedCountryCode) || defaultCountry

    React.useEffect(() => {
        if (!value) {
            setPhoneNumber("")
            return
        }

        // Attempt to match longest matching dial code
        // match from ALL countries, not just sorted ones, though they are the same set
        const match = [...COUNTRIES]
            .sort((a, b) => b.dial_code.length - a.dial_code.length)
            .find(c => value.startsWith(c.dial_code))

        if (match) {
            setSelectedCountryCode(match.code)
            // Store only the number part locally to avoid duplication in input
            setPhoneNumber(value.slice(match.dial_code.length))
        } else {
            setPhoneNumber(value)
        }
    }, [value])


    const handleCountryChange = (code: string) => {
        const country = COUNTRIES.find(c => c.code === code)
        if (country) {
            setSelectedCountryCode(code)
            // Trigger change with new code + existing number
            onChange(`${country.dial_code}${phoneNumber}`)
        }
    }

    const handleNumberChange = (e: React.ChangeEvent<HTMLInputElement>) => {
        const num = e.target.value.replace(/[^0-9]/g, "")
        setPhoneNumber(num)
        onChange(`${selectedCountry.dial_code}${num}`)
    }

    return (
        <div className="flex gap-2">
            <Select
                value={selectedCountryCode}
                onValueChange={handleCountryChange}
                disabled={disabled}
            >
                <SelectTrigger className="w-[140px]">
                    <div className="flex items-center gap-2 overflow-hidden">
                        <img
                            src={`https://flagcdn.com/w20/${selectedCountry.code.toLowerCase()}.png`}
                            srcSet={`https://flagcdn.com/w40/${selectedCountry.code.toLowerCase()}.png 2x`}
                            width="20"
                            height="15"
                            alt={selectedCountry.name}
                            className="shrink-0 object-contain"
                        />
                        <span className="truncate">{selectedCountry.dial_code}</span>
                    </div>
                </SelectTrigger>
                <SelectContent className="max-h-[300px] w-[280px]">
                    {sortedCountries.map((country) => (
                        <SelectItem key={country.code} value={country.code}>
                            <div className="flex items-center gap-2">
                                <img
                                    src={`https://flagcdn.com/w20/${country.code.toLowerCase()}.png`}
                                    srcSet={`https://flagcdn.com/w40/${country.code.toLowerCase()}.png 2x`}
                                    width="20"
                                    height="15"
                                    alt={country.name}
                                    className="shrink-0 object-contain"
                                />
                                <span className="font-mono text-muted-foreground w-12 text-xs">{country.dial_code}</span>
                                <span className="truncate flex-1">{country.name}</span>
                            </div>
                        </SelectItem>
                    ))}
                </SelectContent>
            </Select>

            <Input
                type="tel"
                placeholder="Número"
                value={phoneNumber}
                onChange={handleNumberChange}
                disabled={disabled}
                className="flex-1"
            />
        </div>
    )
}
