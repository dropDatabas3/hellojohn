/**
 * PhoneInput Component — Design System (Professional)
 *
 * International phone number input with validation.
 * Uses libphonenumber-js for accurate parsing and formatting.
 */

import * as React from "react"
import { parsePhoneNumber, CountryCode, AsYouType } from "libphonenumber-js"
import { cn } from "../utils/cn"
import { Input } from "../core/input"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "../core/select"

export interface PhoneInputProps {
  value?: string
  onChange?: (value: string) => void
  defaultCountry?: CountryCode
  error?: string
  disabled?: boolean
  className?: string
}

const POPULAR_COUNTRIES: CountryCode[] = ['US', 'GB', 'CA', 'AU', 'DE', 'FR', 'ES', 'IT', 'MX', 'BR', 'AR']

export function PhoneInput({
  value = "",
  onChange,
  defaultCountry = "US",
  error,
  disabled,
  className,
}: PhoneInputProps) {
  const [country, setCountry] = React.useState<CountryCode>(defaultCountry)
  const [localValue, setLocalValue] = React.useState(value)

  React.useEffect(() => {
    setLocalValue(value)
  }, [value])

  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const input = e.target.value
    const asYouType = new AsYouType(country)
    const formatted = asYouType.input(input)

    setLocalValue(formatted)
    onChange?.(formatted)
  }

  const isValid = React.useMemo(() => {
    try {
      if (!localValue) return true
      const phoneNumber = parsePhoneNumber(localValue, country)
      return phoneNumber?.isValid() ?? false
    } catch {
      return false
    }
  }, [localValue, country])

  return (
    <div className={cn("space-y-2", className)}>
      <div className="flex gap-2">
        <Select value={country} onValueChange={(v) => setCountry(v as CountryCode)} disabled={disabled}>
          <SelectTrigger className="w-[120px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {POPULAR_COUNTRIES.map((code) => (
              <SelectItem key={code} value={code}>
                {getFlagEmoji(code)} {code}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>

        <Input
          type="tel"
          value={localValue}
          onChange={handleChange}
          disabled={disabled}
          className={cn(
            "flex-1",
            !isValid && localValue && "border-destructive focus-visible:ring-destructive"
          )}
          placeholder="+1 (555) 000-0000"
        />
      </div>

      {error && (
        <p className="text-xs text-destructive">{error}</p>
      )}
    </div>
  )
}

function getFlagEmoji(countryCode: string): string {
  const codePoints = countryCode
    .toUpperCase()
    .split('')
    .map((char) => 127397 + char.charCodeAt(0))
  return String.fromCodePoint(...codePoints)
}
