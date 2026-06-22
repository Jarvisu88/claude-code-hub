/**
 * @vitest-environment happy-dom
 */

import { act } from "react";
import { createRoot } from "react-dom/client";
import { describe, expect, it, vi } from "vitest";

vi.mock("next-intl", () => ({
  useTranslations: () => (key: string) => key,
}));

vi.mock("@/components/ui/button", () => ({
  Button: ({ children, ...props }: any) => <button {...props}>{children}</button>,
}));

vi.mock("@/components/ui/separator", () => ({
  Separator: () => <span data-testid="separator" />,
}));

vi.mock("lucide-react", () => ({
  Activity: () => <span />,
  FlaskConical: () => <span />,
  Pencil: () => <span />,
  RotateCcw: () => <span />,
  Trash2: () => <span />,
}));

import {
  ProviderBatchActions,
  type BatchActionMode,
} from "@/app/[locale]/settings/providers/_components/batch-edit/provider-batch-actions";

function render(node: React.ReactNode) {
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);

  act(() => {
    root.render(node);
  });

  return {
    container,
    unmount: () => {
      act(() => root.unmount());
      container.remove();
    },
  };
}

describe("ProviderBatchActions", () => {
  it("exposes automatic upstream sync and Mini probe switch actions", () => {
    const onAction = vi.fn<(mode: BatchActionMode) => void>();
    const { container, unmount } = render(
      <ProviderBatchActions
        selectedCount={2}
        isVisible={true}
        onAction={onAction}
        onClose={vi.fn()}
      />
    );

    const buttons = Array.from(container.querySelectorAll("button"));
    const rateSyncOnButton = buttons.find((button) =>
      button.textContent?.includes("actions.rateSyncAutoOn")
    );
    const rateSyncOffButton = buttons.find((button) =>
      button.textContent?.includes("actions.rateSyncAutoOff")
    );
    const miniProbeOnButton = buttons.find((button) =>
      button.textContent?.includes("actions.miniProbeAutoOn")
    );
    const miniProbeOffButton = buttons.find((button) =>
      button.textContent?.includes("actions.miniProbeAutoOff")
    );

    expect(rateSyncOnButton).toBeTruthy();
    expect(rateSyncOffButton).toBeTruthy();
    expect(miniProbeOnButton).toBeTruthy();
    expect(miniProbeOffButton).toBeTruthy();

    act(() => {
      rateSyncOnButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      rateSyncOffButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      miniProbeOnButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
      miniProbeOffButton?.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });

    expect(onAction).toHaveBeenCalledWith("rateSyncAutoOn");
    expect(onAction).toHaveBeenCalledWith("rateSyncAutoOff");
    expect(onAction).toHaveBeenCalledWith("miniProbeAutoOn");
    expect(onAction).toHaveBeenCalledWith("miniProbeAutoOff");

    unmount();
  });
});
