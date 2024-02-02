import { useRequestHeaders } from "#app";
import { extractCookie } from "~/utils/cookie";
import { resolveAcceptLanguage } from "~/utils/resolve-accept-language/resolve-accept-language";

const en = {
  common: {
    user_id: 'Username',
    user_role: 'Role',
    user_role_admin: 'Admin',
    user_role_standard: 'Standard',
    user_role_disabled: 'Disabled',
    server_user: 'Server User',
    server_id: 'Server Name',
    server_address: 'Server Address',
    created_at: 'Created At',
    ssh_key_id: 'Fingerprint',
    ssh_key_display_name: 'Display Name',
    edit: 'Edit',
    delete: 'Delete',
    username: 'Username',
    password: 'Password',
    submit: 'Submit',
    sign_out: 'Sign Out',
    sign_in: 'Sign In'
  },
  dashboard: {
    command_example: 'Command Example',
    intro: 'Execute ssh command with format <code>ssh SERVER_USER@SERVER_NAME@BUNKER_ADDRESS</code>, the <code>SERVER_USER@SERVER_NAME</code> part will be sent to Bunker server as user field to determine the target server',
    title: 'Dashboard'
  },
  servers: {
    add_update_server: 'Add / Update Server',
    title: 'Servers',
    input_server_id: 'Input server name here',
    input_server_address: 'Input server address here',
    view_authorized_keys: 'View Authorized Keys',
    intro_authorized_keys: 'To allow Bunker to relay SSH connections to this server, please add the following public key to the server user\'s <code>$HOME/.ssh/authorized_keys</code> file'
  },
  users: {
    title: 'Users',
    add_update_user: 'Add / Update User',
    input_user_id: 'Input user name here',
    input_password: 'Input password here',
    revoke_admin: 'Revoke Admin',
    assign_admin: 'Assign Admin',
    disable: 'Disable',
    enable: 'Enable',
  },
  grants: {
    title: 'Grants',
    add_grant: 'Add Grant',
    intro_asterisk: 'The asterisk (*) is a wildcard that matches any server user or server name',
  },
  ssh_keys: {
    title: 'SSH Keys',
    add_ssh_key: 'Add SSH Key',
    public_key: 'Public Key',
    input_display_name: 'Input display name here',
    input_public_key: 'Input public key here',
  },
  profile: {
    title: 'Profile',
    update_password: 'Update Password',
    password_updated: 'Password updated successfully',
    old_password: 'Old Password',
    input_old_password: 'Input old password here',
    new_password: 'New Password',
    input_new_password: 'Input new password here',
    repeat_password: 'Repeat Password',
    input_repeat_password: 'Input repeat password here',
  },
  lastwill: "Alive?",
  pronouns: "him",
  donation: "donation",
  copyright: "All Rights Reserved",
  location: "Shenzhen, China",
};

const zh: typeof en = {
  common: {
    user_id: '用户名',
    user_role: '角色',
    user_role_admin: '管理员',
    user_role_standard: '普通用户',
    user_role_disabled: '已禁用',
    server_user: '服务器用户',
    server_id: '服务器名称',
    created_at: '创建时间',
    server_address: '服务器地址',
    ssh_key_display_name: '显示名称',
    ssh_key_id: '指纹',
    edit: '编辑',
    delete: '删除',
    submit: '提交',
    username: '用户名',
    password: '密码',
    sign_out: '登出',
    sign_in: '登录'
  },
  dashboard: {
    command_example: '命令示例',
    intro: '使用格式 <code>ssh 用户@服务器@BUNKER地址</code> 执行 ssh 命令<br/><br/>其中 <code>服务器用户@服务器名称</code> 部分会以用户字段发送到 Bunker 服务器，用来判断要连接的目标服务器',
    title: '工作台'
  },
  servers: {
    add_update_server: '添加 / 更新服务器',
    title: '服务器管理',
    input_server_id: '在此输入服务器名称',
    input_server_address: '在此输入服务器地址',
    view_authorized_keys: '查看公钥',
    intro_authorized_keys: '为了让服务器的 SSH 连接可以被 Bunker 中继，请将以下公钥添加到目标服务器用户的 <code>$HOME/.ssh/authorized_keys</code> 文件中',
  },
  users: {
    title: '用户管理',
    add_update_user: '添加 / 更新用户',
    input_user_id: '在此输入用户名',
    input_password: '在此输入密码',
    assign_admin: '授予管理员',
    revoke_admin: '撤销管理员',
    enable: '启用',
    disable: '禁用',
  },
  grants: {
    title: '授权管理',
    add_grant: '添加授权',
    intro_asterisk: '星号 (*) 是通配符，匹配任意服务器用户或服务器名称',
  },
  ssh_keys: {
    title: 'SSH 公钥',
    add_ssh_key: '添加 SSH 公钥',
    public_key: '公钥',
    input_display_name: '在此输入显示名称',
    input_public_key: '在此输入公钥',
  },
  profile: {
    title: '个人资料',
    update_password: '更新密码',
    password_updated: '密码更新成功',
    old_password: '旧密码',
    input_old_password: '在此输入旧密码',
    new_password: '新密码',
    input_new_password: '在此输入新密码',
    repeat_password: '重复密码',
    input_repeat_password: '在此输入重复密码',
  },
  lastwill: "存活?",
  pronouns: "他",
  location: "深圳，中国",
  donation: "赞助",
  copyright: "保留所有权利",
};

const i18nData: {
  langs: Array<string>;
  langNames: Record<string, string>;
  messages: any;
} = {
  langs: ["en-US", "zh-CN"],
  langNames: {
    "en-US": "English",
    "zh-CN": "简体中文",
  },
  messages: { "en-US": en, "zh-CN": zh },
};

const i18nDetectors: Array<() => string | undefined> = [
  function (): string | undefined {
    // server cookie
    const cookie = useRequestHeaders(["cookie"])["cookie"] || "";
    if (cookie) {
      return extractCookie(cookie, "lang");
    }
    return;
  },
  function (): string | undefined {
    // client cookie
    if (typeof window !== "undefined") {
      return extractCookie(window.document.cookie, "lang");
    }
    return;
  },
  function (): string | undefined {
    // server accept-language
    let accept =
      useRequestHeaders(["accept-language"])["accept-language"] || "";
    if (accept) {
      return resolveAcceptLanguage(accept, i18nData.langs, i18nData.langs[0]);
    }
    return;
  },
  function (): string | undefined {
    // client language
    if (typeof window !== "undefined") {
      let accept = window.navigator.languages.join(", ");
      if (accept) {
        return resolveAcceptLanguage(accept, i18nData.langs, i18nData.langs[0]);
      }
    }
    return;
  },
];
export default defineNuxtPlugin((nuxtApp) => {
  let lang = "";

  for (const detector of i18nDetectors) {
    lang = detector() || "";
    if (lang) {
      break;
    }
  }

  if (!lang) {
    lang = i18nData.langs[0];
  }

  let langName = i18nData.langNames[lang];

  return {
    provide: {
      lang,
      langName,
      langs: i18nData.langs,
      langNames: i18nData.langNames,
      t: (key: string): string => {
        const splits = key.split(".");
        // @ts-ignore
        let v = i18nData.messages[lang];
        for (const item of splits) {
          v = (v || {})[item];
          if (!v) {
            return "<MISSING: " + key + ">";
          }
        }
        return v;
      },
    },
  };
});
