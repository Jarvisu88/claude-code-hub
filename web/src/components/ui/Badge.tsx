import type { HTMLAttributes, ReactNode } from "react";
import { cn } from "@/lib/utils";

interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: "default" | "success" | "warning" | "danger" | "secondary";
  children: ReactNode;
}

function Badge({
  className,
  variant = "default",
  children,
  ...props
}: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium",
        {
          "bg-[var(--color-primary-light)] text-[var(--color-primary)]":
            variant === "default",
          "bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400":
            variant === "success",
          "bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400":
            variant === "warning",
          "bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400":
            variant === "danger",
          "bg-[var(--color-bg-tertiary)] text-[var(--color-text-secondary)]":
            variant === "secondary",
        },
        className,
      )}
      {...props}
    >
      {children}
    </span>
  );
}

export { Badge, type BadgeProps };
