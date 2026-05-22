import { cn } from "@/lib/utils";

interface SpinnerProps {
  size?: "sm" | "md" | "lg";
  className?: string;
}

function Spinner({ size = "md", className }: SpinnerProps) {
  return (
    <div
      className={cn(
        "animate-spin rounded-full border-2 border-[var(--color-border)] border-t-[var(--color-primary)]",
        {
          "h-4 w-4": size === "sm",
          "h-6 w-6": size === "md",
          "h-10 w-10 border-[3px]": size === "lg",
        },
        className,
      )}
      role="status"
      aria-label="Loading"
    />
  );
}

export { Spinner, type SpinnerProps };
