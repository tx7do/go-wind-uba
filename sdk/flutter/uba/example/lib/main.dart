import 'package:flutter/material.dart';

import 'package:uba_flutter/uba_flutter.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // 1. 初始化（必须在 runApp 之前 await，device id 恢复是异步的）
  final client = await UbaClient.init(UbaConfig(
    appId: 'your_app_id',
    appSecret: 'your_app_secret',
    endpoint: 'http://collector.your-domain.com:5700',
    appVersion: '1.0.0',
    channel: 'official',
    debug: true, // 联调期打开，正式发布关闭
    // 可选能力（默认即推荐值）：
    // autoTrackLaunch: true,      // 首装自动埋 app_launch
    // sampleRate: 1.0,            // 会话级确定性采样
    // requireConsent: true,       // GDPR 场景：授权后才采集
    // sessionTimeoutMs: 30000,    // 后台 30s 回前台 = 新会话
  ));

  // 2. 注册生命周期观察（后台自动 flush + 会话轮换）
  UbaLifecycleObserver(client).register();

  runApp(UbaApp(client: client));
}

class UbaApp extends StatelessWidget {
  const UbaApp({super.key, required this.client});

  final UbaClient client;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'UBA Flutter Demo',
      // 3. 页面浏览自动埋点（page_view，含来源页停留时长）
      navigatorObservers: [UbaScreenObserver(client)],
      routes: {
        '/home': (_) => HomePage(client: client),
        '/detail': (_) => const DetailPage(),
      },
    );
  }
}

class HomePage extends StatefulWidget {
  const HomePage({super.key, required this.client});

  final UbaClient client;

  @override
  State<HomePage> createState() => _HomePageState();
}

class _HomePageState extends State<HomePage> {
  @override
  void initState() {
    super.initState();
    _bootstrap();
  }

  Future<void> _bootstrap() async {
    final client = widget.client;

    // 4. 登录态绑定（登录成功后调用）
    client.identify(10001);

    // 5. 拉取 AB 实验开关；getFlag 自动上报一次 $feature_flag 曝光
    await client.loadFlags();
    if (mounted) {
      final variant = client.getFlag('home_banner');
      if (client.isFeatureEnabled('home_banner')) {
        debugPrint('命中实验变体: $variant');
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('UBA Demo · Home')),
      body: Center(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            FilledButton(
              onPressed: () => Navigator.pushNamed(context, '/detail'),
              child: const Text('进入详情页（自动埋 page_view）'),
            ),
            const SizedBox(height: 12),
            FilledButton(
              onPressed: () => widget.client.track(
                'button_click',
                properties: {'scene': 'home', 'button': 'pay'},
                options: const TrackOptions(
                  eventCategory: 'interaction',
                  eventAction: 'click',
                  objectType: 'button',
                  objectId: 'pay_btn',
                ),
              ),
              child: const Text('自定义事件（track）'),
            ),
            const SizedBox(height: 12),
            FilledButton(
              onPressed: () => widget.client.trackRisk(
                'login_failed',
                const RiskEvent(
                  riskType: 'login_anomaly',
                  riskLevel: 'HIGH',
                  riskScore: 88.5,
                  ruleName: '1分钟内连续失败5次',
                ),
                properties: {'try_count': '5'},
              ),
              child: const Text('风险事件（trackRisk）'),
            ),
            const SizedBox(height: 12),
            FilledButton(
              onPressed: () async {
                final r = await widget.client.flush();
                if (context.mounted) {
                  ScaffoldMessenger.of(context).showSnackBar(
                    SnackBar(content: Text(r.toString())),
                  );
                }
              },
              child: const Text('手动 flush'),
            ),
          ],
        ),
      ),
    );
  }
}

class DetailPage extends StatelessWidget {
  const DetailPage({super.key});

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('UBA Demo · Detail')),
      body: const Center(child: Text('返回 Home 观察埋点')),
    );
  }
}
