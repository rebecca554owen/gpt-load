import type { Group, SubGroupInfo } from "@/types/models";

/**
 * 将 camelCase、snake_case 或 kebab-case 格式的字符串
 * 转换为更易读的格式，使用空格分隔单词并首字母大写。
 *
 * @param name 输入字符串
 * @returns 格式化后的字符串
 *
 * @example
 * formatDisplayName("myGroupName")      // "My Group Name"
 * formatDisplayName("my_group_name")    // "My Group Name"
 * formatDisplayName("my-group-name")    // "My Group Name"
 * formatDisplayName("MyGroup")          // "My Group"
 */
export function formatDisplayName(name: string): string {
  if (!name) {
    return "";
  }

  // 将 snake_case 和 kebab-case 替换为空格，并在 camelCase 的大写字母前添加空格
  const result = name.replace(/[_-]/g, " ").replace(/([a-z])([A-Z])/g, "$1 $2");

  // 将每个单词的首字母大写
  return result
    .split(" ")
    .filter(word => word.length > 0)
    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
    .join(" ");
}

/**
 * 获取分组或子分组的显示名称，如果没有则使用格式化后的名称。
 * @param item 分组或子分组对象
 * @returns 分组的显示名称
 */
export function getGroupDisplayName(item: Group | SubGroupInfo): string {
  if ("group" in item && item.group) {
    const group = item.group as Group;
    return group.display_name || formatDisplayName(group.name);
  }
  const group = item as Group;
  return group.display_name || formatDisplayName(group.name);
}

/**
 * 对长密钥字符串进行掩码处理以便显示。
 * @param key 密钥字符串
 * @returns 掩码后的密钥
 */
export function maskKey(key: string): string {
  if (!key || key.length <= 8) {
    return key || "";
  }
  return `${key.substring(0, 4)}...${key.substring(key.length - 4)}`;
}

/**
 * 对逗号分隔的密钥字符串进行掩码处理。
 * @param keys 逗号分隔的密钥字符串
 * @returns 掩码后的密钥字符串
 */
export function maskProxyKeys(keys: string): string {
  if (!keys) {
    return "";
  }
  return keys
    .split(",")
    .map(key => maskKey(key.trim()))
    .join(", ");
}
