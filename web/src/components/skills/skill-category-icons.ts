import { Boxes, Clapperboard, Megaphone, Palette, Puzzle, ShoppingBag, type LucideIcon } from "lucide-react";

const skillCategoryIcons: Record<string, LucideIcon> = {
    drama: Clapperboard,
    ecommerce: ShoppingBag,
    creative: Palette,
    social: Megaphone,
    others: Puzzle,
};

export function skillCategoryIconOf(value: string): LucideIcon {
    return skillCategoryIcons[value] ?? Boxes;
}
