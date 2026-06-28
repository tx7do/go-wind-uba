using UnityEngine;

namespace Uba.Unity
{
    /// <summary>
    /// Unity 上下文提供者：用 UnityEngine.SystemInfo 采集精确的设备/平台信息。
    /// </summary>
    public class UnityContextProvider : IContextProvider
    {
        private readonly string _deviceId;
        private readonly string _sessionId;
        private readonly string _platform;

        public UnityContextProvider()
        {
            // 设备 ID：优先用 SystemInfo.uniqueIdentifier（注意隐私合规，部分平台受限），
            // 回退到 Playerprefs 持久化的 GUID。
            _deviceId = LoadOrCreateDeviceId();
            _sessionId = System.Guid.NewGuid().ToString("D");
            _platform = DetectPlatform();
        }

        public string GetDeviceId() => _deviceId;
        public string GetSessionId() => _sessionId;
        public string GetPlatform() => _platform;

        public ClientInfo GetClientInfo()
        {
            var info = new ClientInfo
            {
                UserAgent = $"Unity/{Application.unityVersion} ({SystemInfo.operatingSystem})",
            };
            return info;
        }

        private static string LoadOrCreateDeviceId()
        {
            const string key = "__uba_device_id__";
            var existing = PlayerPrefs.GetString(key, "");
            if (!string.IsNullOrEmpty(existing))
            {
                return existing;
            }
            var id = System.Guid.NewGuid().ToString("D");
            PlayerPrefs.SetString(key, id);
            PlayerPrefs.Save();
            return id;
        }

        private static string DetectPlatform()
        {
#if UNITY_EDITOR
            return "unity_editor";
#elif UNITY_IOS
            return "ios";
#elif UNITY_ANDROID
            return "android";
#elif UNITY_WEBGL
            return "webgl";
#elif UNITY_STANDALONE_WIN
            return "windows";
#elif UNITY_STANDALONE_OSX
            return "macos";
#elif UNITY_STANDALONE_LINUX
            return "linux";
#else
            return "unity";
#endif
        }
    }
}
