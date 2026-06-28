using System.Collections;
using UnityEngine;

namespace Uba.Unity
{
    /// <summary>
    /// Unity 便捷封装：挂在场景中的持久 GameObject 上，自动初始化 SDK，
    /// 在 OnApplicationQuit / OnApplicationPause 时触发 flush。
    /// </summary>
    public class UbaUnityBehaviour : MonoBehaviour
    {
        private static UbaClient? _client;

        /// <summary>全局访问入口（Init 之后可用）</summary>
        public static UbaClient Client => _client ?? throw new System.InvalidOperationException("UbaUnityBehaviour not initialized");

        [Header("SDK 配置")]
        [Tooltip("collector 服务地址，如 http://localhost:5700")]
        public string endpoint = "http://localhost:5700";
        public string appId = "demo_app_001";
        public string appSecret = "demo_secret_123456";
        [Tooltip("缓冲达到该数量触发批量上报")]
        public int batchSize = 20;
        [Tooltip("定时上报间隔（毫秒）")]
        public int flushIntervalMs = 5000;
        public bool debug = false;

        /// <summary>初始化 SDK（也可在 Awake 中自动调用）</summary>
        public void Init()
        {
            if (_client != null) return;
            var transport = new UnityWebRequestTransport(this);
            var context = new UnityContextProvider();
            _client = new UbaClient(new UbaConfig
            {
                AppId = appId,
                AppSecret = appSecret,
                Endpoint = endpoint,
                BatchSize = batchSize,
                FlushInterval = flushIntervalMs,
                Debug = debug,
            }, transport, context);
            Debug.Log($"[UBA] initialized: endpoint={endpoint} appId={appId}");
        }

        void Awake()
        {
            // 保证跨场景不销毁
            DontDestroyOnLoad(gameObject);
            Init();
        }

        void OnApplicationPause(bool paused)
        {
            // 切后台时尽力 flush
            if (paused) StartCoroutine(FlushCoroutine());
        }

        void OnApplicationQuit()
        {
            // 退出时同步等待一次 flush（注意 WebGL 下 OnApplicationQuit 不可靠）
            StartCoroutine(FlushCoroutine());
        }

        void OnDestroy()
        {
            _client?.Dispose();
            _client = null;
        }

        private IEnumerator FlushCoroutine()
        {
            if (_client == null) yield break;
            var task = _client.FlushAsync();
            while (!task.IsCompleted) yield return null;
            if (task.IsFaulted && task.Exception != null)
            {
                Debug.LogWarning($"[UBA] flush failed: {task.Exception}");
            }
        }

        // ── 便捷静态方法 ──

        public static void Track(string eventName, System.Collections.Generic.Dictionary<string, string>? properties = null, TrackOptions? options = null)
            => Client.Track(eventName, properties, options);

        public static void TrackRisk(string eventName, RiskEvent risk, TrackOptions? options = null)
            => Client.TrackRisk(eventName, risk, options);

        public static void Identify(uint userId) => Client.Identify(userId);
    }
}
