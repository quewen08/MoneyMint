<template>
  <!-- 登录页面保持独立布局，不使用默认布局 -->
  <NuxtLayout :name="false">
    <div class="min-h-screen bg-gradient-to-br from-blue-50 via-white to-indigo-50 dark:from-gray-900 dark:via-gray-800 dark:to-gray-900 flex items-center justify-center px-4 py-8">
      <div class="bg-white dark:bg-gray-800 p-8 rounded-2xl shadow-xl w-full max-w-md animate-fade-in">
        <!-- Logo 和标题 -->
        <div class="text-center mb-8">
          <div class="w-16 h-16 bg-gradient-to-br from-blue-500 to-indigo-600 rounded-2xl mx-auto mb-4 flex items-center justify-center shadow-lg">
            <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
            </svg>
          </div>
          <h1 class="text-3xl font-bold text-gray-800 dark:text-white mb-2">
            MoneyMint
          </h1>
          <p class="text-gray-500 dark:text-gray-400 text-sm">轻松管理您的财务</p>
        </div>

        <!-- 选项卡导航 -->
        <div class="flex border-b border-gray-200 dark:border-gray-700 mb-6">
          <button
            :class="[
              'flex-1 py-3 px-4 font-medium text-sm transition-all duration-200',
              activeTab === 'login'
                ? 'border-b-2 border-primary text-primary dark:text-blue-400'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300',
            ]"
            @click="activeTab = 'login'"
          >
            登录
          </button>
          <button
            v-if="showRegisterTab"
            :class="[
              'flex-1 py-3 px-4 font-medium text-sm transition-all duration-200',
              activeTab === 'register'
                ? 'border-b-2 border-primary text-primary dark:text-blue-400'
                : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300',
            ]"
            @click="activeTab = 'register'"
          >
            注册
          </button>
        </div>

        <!-- 登录表单 -->
        <div v-if="activeTab === 'login'" class="animate-slide-up">
          <form @submit.prevent="handleLogin" class="space-y-5">
            <div>
              <label
                for="login-username"
                class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
                >用户名</label
              >
              <div class="relative">
                <input
                  type="text"
                  id="login-username"
                  v-model="loginForm.username"
                  required
                  class="input pl-10"
                  placeholder="请输入用户名"
                />
                <div class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400">
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"></path>
                  </svg>
                </div>
              </div>
            </div>

            <div>
              <label
                for="login-password"
                class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
                >密码</label
              >
              <div class="relative">
                <input
                  :type="showLoginPassword ? 'text' : 'password'"
                  id="login-password"
                  v-model="loginForm.password"
                  required
                  class="input pr-12"
                  placeholder="请输入密码"
                />
                <button
                  type="button"
                  @click="showLoginPassword = !showLoginPassword"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 transition-colors touch-target"
                >
                  <svg v-if="!showLoginPassword" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"></path>
                  </svg>
                  <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7a9.97 9.97 0 01-1.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7a9.97 9.97 0 01-1.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242"></path>
                  </svg>
                </button>
              </div>
            </div>

            <div
              v-if="error"
              class="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 px-4 py-3 rounded-lg flex items-center gap-2"
            >
              <svg class="w-5 h-5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
              </svg>
              <span class="text-sm">{{ error }}</span>
            </div>

            <button
              type="submit"
              class="btn btn-primary w-full py-3 text-base font-semibold"
              :disabled="loading"
            >
              <span v-if="loading" class="inline-flex items-center gap-2">
                <span class="inline-block animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent"></span>
                登录中...
              </span>
              <span v-else>登录</span>
            </button>
          </form>
        </div>

        <!-- 注册表单 -->
        <div v-else-if="showRegisterTab" class="animate-slide-up">
          <form @submit.prevent="handleRegister" class="space-y-5">
            <div>
              <label
                for="register-username"
                class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
                >用户名</label
              >
              <div class="relative">
                <input
                  type="text"
                  id="register-username"
                  v-model="registerForm.username"
                  required
                  class="input pl-10"
                  placeholder="请输入用户名"
                />
                <div class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400">
                  <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"></path>
                  </svg>
                </div>
              </div>
            </div>

            <div>
              <label
                for="register-password"
                class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
                >密码</label
              >
              <div class="relative">
                <input
                  :type="showRegisterPassword ? 'text' : 'password'"
                  id="register-password"
                  v-model="registerForm.password"
                  required
                  minlength="6"
                  class="input pr-12"
                  placeholder="请输入至少6位密码"
                />
                <button
                  type="button"
                  @click="showRegisterPassword = !showRegisterPassword"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 transition-colors touch-target"
                >
                  <svg v-if="!showRegisterPassword" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7a9.97 9.97 0 01-1.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7a9.97 9.97 0 01-1.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242"></path>
                  </svg>
                  <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7a9.97 9.97 0 01-1.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242"></path>
                  </svg>
                </button>
              </div>
            </div>

            <div>
              <label
                for="register-confirm-password"
                class="block text-sm font-medium text-gray-700 dark:text-gray-300 mb-2"
                >确认密码</label
              >
              <div class="relative">
                <input
                  :type="showRegisterConfirmPassword ? 'text' : 'password'"
                  id="register-confirm-password"
                  v-model="registerForm.confirmPassword"
                  required
                  class="input pr-12"
                  placeholder="请再次输入密码"
                />
                <button
                  type="button"
                  @click="showRegisterConfirmPassword = !showRegisterConfirmPassword"
                  class="absolute right-3 top-1/2 -translate-y-1/2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-200 transition-colors touch-target"
                >
                  <svg v-if="!showRegisterConfirmPassword" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"></path>
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7a9.97 9.97 0 01-1.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7a9.97 9.97 0 01-1.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242"></path>
                  </svg>
                  <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7a9.97 9.97 0 01-1.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7a9.97 9.97 0 01-1.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242"></path>
                  </svg>
                </button>
              </div>
            </div>

            <div
              v-if="error"
              class="bg-red-50 dark:bg-red-900/30 border border-red-200 dark:border-red-800 text-red-700 dark:text-red-300 px-4 py-3 rounded-lg flex items-center gap-2"
            >
              <svg class="w-5 h-5 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
              </svg>
              <span class="text-sm">{{ error }}</span>
            </div>

            <button
              type="submit"
              class="btn btn-primary w-full py-3 text-base font-semibold"
              :disabled="loading"
            >
              <span v-if="loading" class="inline-flex items-center gap-2">
                <span class="inline-block animate-spin rounded-full h-4 w-4 border-2 border-white border-t-transparent"></span>
                注册中...
              </span>
              <span v-else>注册</span>
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
import { useNuxtApp } from '#app';

const router = useRouter();
const { $api } = useNuxtApp();

// 注册状态控制
const showRegisterTab = ref(true);

// 选项卡控制
const activeTab = ref("login");
const loading = ref(false);
const error = ref("");

// 密码可见性控制
const showLoginPassword = ref(false);
const showRegisterPassword = ref(false);
const showRegisterConfirmPassword = ref(false);

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
    const status = await $api.checkRegistrationStatus();
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
    await $api.login(loginForm.value.username, loginForm.value.password);
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
    await $api.register(
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
if ($api.user.value) {
  router.push("/");
}
</script>