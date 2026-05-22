import { type InputHTMLAttributes, forwardRef } from "react";
import { cn } from "@/lib/utils";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
}

const Input = forwardRef<HTMLInputElement, InputProps>(
  ({ className, label, error, id, ...props }, ref) => {
    const inputId = id ?? label?.toLowerCase().replace(/\s+/g, "-");

    return (
      <div className="w-full">
        {label && (
          <label
            htmlFor={inputId}
            className="mb-1.5 block text-sm font-medium text-[var(--color-text)]"
          >
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={inputId}
          className={cn(
            "flex h-10 w-full rounded-md border border-[var(--color-border)] bg-[var(--color-bg)] px-3 py-2 text-sm",
            "text-[var(--color-text)] placeholder:text-[var(--color-text-tertiary)]",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-primary)] focus-visible:border-transparent",
            "disabled:cursor-not-allowed disabled:opacity-50",
            "transition-colors",
            error && "border-[var(--color-danger)] focus-visible:ring-[var(--color-danger)]",
            className,
          )}
          {...props}
        />
        {error && (
          <p className="mt-1.5 text-sm text-[var(--color-danger)]">{error}</p>
        )}
      </div>
    );
  },
);

Input.displayName = "Input";

export { Input, type InputProps };
