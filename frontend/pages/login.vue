<template>
  <!-- 登录页面保持独立布局，不使用默认布局 -->
  <NuxtLayout :name="false">
    <div class="min-h-screen bg-gray-100 flex items-center justify-center">
      <div class="bg-white p-8 rounded-lg shadow-lg w-full max-w-md">
        <h1 class="text-3xl font-bold text-center text-gray-800 mb-8">
          MoneyMint
        </h1>

        <!-- 选项卡导航 -->
        <div class="flex border-b mb-6">
          <button
            :class="[
              'flex-1 py-2 px-4 font-medium text-sm',
              activeTab === 'login'
                ? 'border-b-2 border-blue-500 text-blue-600'
                : 'text-gray-500 hover:text-gray-700',
            ]"
            @click="activeTab = 'login'"
          >
            登录
          </button>
          <button
            v-if="showRegisterTab"
            :class="[
              'flex-1 py-2 px-4 font-medium text-sm',
              activeTab === 'register'
                ? 'border-b-2 border-blue-500 text-blue-600'
                : 'text-gray-500 hover:text-gray-700',
            ]"
            @click="activeTab = 'register'"
          >
            注册
          </button>
        </div>

        <!-- 登录表单 -->
        <div v-if="activeTab === 'login'">
          <form @submit.prevent="handleLogin" class="space-y-4">
            <div>
              <label
                for="login-username"
                class="block text-sm font-medium text-gray-700 mb-1"
                >用户名</label
              >
              <input
                type="text"
                id="login-username"
                v-model="loginForm.username"
                required
                class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="请输入用户名"
              />
            </div>

            <div>
              <label
                for="login-password"
                class="block text-sm font-medium text-gray-700 mb-1"
                >密码</label
              >
              <input
                type="password"
                id="login-password"
                v-model="loginForm.password"
                required
                class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="请输入密码"
              />
            </div>

            <div
              v-if="error"
              class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded-md"
            >
              {{ error }}
            </div>

            <button
              type="submit"
              class="w-full bg-blue-500 hover:bg-blue-600 text-white font-medium py-2 px-4 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
              :disabled="loading"
            >
              {{ loading ? "登录中..." : "登录" }}
            </button>
          </form>
        </div>

        <!-- 注册表单 -->
        <div v-else-if="showRegisterTab">
          <form @submit.prevent="handleRegister" class="space-y-4">
            <div>
              <label
                for="register-username"
                class="block text-sm font-medium text-gray-700 mb-1"
                >用户名</label
              >
              <input
                type="text"
                id="register-username"
                v-model="registerForm.username"
                required
                class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="请输入用户名"
              />
            </div>

            <div>
              <label
                for="register-password"
                class="block text-sm font-medium text-gray-700 mb-1"
                >密码</label
              >
              <input
                type="password"
                id="register-password"
                v-model="registerForm.password"
                required
                minlength="6"
                class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="请输入至少6位密码"
              />
            </div>

            <div>
              <label
                for="register-confirm-password"
                class="block text-sm font-medium text-gray-700 mb-1"
                >确认密码</label
              >
              <input
                type="password"
                id="register-confirm-password"
                v-model="registerForm.confirmPassword"
                required
                class="w-full px-3 py-2 border border-gray-300 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500"
                placeholder="请再次输入密码"
              />
            </div>

            <div
              v-if="error"
              class="bg-red-100 border border-red-400 text-red-700 px-4 py-3 rounded-md"
            >
              {{ error }}
            </div>

            <button
              type="submit"
              class="w-full bg-blue-500 hover:bg-blue-600 text-white font-medium py-2 px-4 rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2"
              :disabled="loading"
            >
              {{ loading ? "注册中..." : "注册" }}
            </button>
          </form>
        </div>
      </div>
    </div>
  </NuxtLayout>
</template>

<script setup lang="ts">
import { ref, onMounted } from "vue";
import { useRouter } from "vue-router";
import { useApi } from "~/composables/useApi";

const router = useRouter();
const api = useApi();

// 注册状态控制
const showRegisterTab = ref(true);

// 选项卡控制
const activeTab = ref("login");
const loading = ref(false);
const error = ref("");

// 表单数据
const loginForm = ref({
  username: "",
  password: "",
});

const registerForm = ref({
  username: "",
  password: "",
  confirmPassword: "",
});

// 检查注册功能是否可用
const checkRegistrationAvailability = async () => {
  try {
    const status = await api.checkRegistrationStatus();
    showRegisterTab.value = status.enabled;
    // 如果注册功能关闭且当前在注册标签页，切换到登录标签页
    if (!status.enabled && activeTab.value === "register") {
      activeTab.value = "login";
    }
  } catch (error) {
    console.error("检查注册状态失败:", error);
    showRegisterTab.value = false;
    // 出错时如果当前在注册标签页，切换到登录标签页
    if (activeTab.value === "register") {
      activeTab.value = "login";
    }
  }
};

// 页面加载时检查注册功能状态
onMounted(() => {
  checkRegistrationAvailability();
});

// 处理登录
const handleLogin = async () => {
  loading.value = true;
  error.value = "";

  try {
    await api.login(loginForm.value.username, loginForm.value.password);
    await router.push("/");
  } catch (err: any) {
    error.value = err.message || "登录失败，请检查用户名和密码";
  } finally {
    loading.value = false;
  }
};

// 处理注册
const handleRegister = async () => {
  loading.value = true;
  error.value = "";

  if (registerForm.value.password !== registerForm.value.confirmPassword) {
    error.value = "两次输入的密码不一致";
    loading.value = false;
    return;
  }

  try {
    await api.register(
      registerForm.value.username,
      registerForm.value.password
    );
    // 注册成功后自动切换到登录选项卡
    activeTab.value = "login";
    loginForm.value.username = registerForm.value.username;
    registerForm.value = {
      username: "",
      password: "",
      confirmPassword: "",
    };
  } catch (err: any) {
    console.log(err);
    error.value = err.message || "注册失败，请稍后重试";
  } finally {
    loading.value = false;
  }
};

// 如果已经登录，跳转到首页
if (api.user.value) {
  router.push("/");
}
</script>