// UbaSubsystem.cpp —— 子系统实现：核心层装配 + FHttpModule 传输桥接。
#include "UbaSubsystem.h"

#include "Engine/Engine.h"
#include "Http.h"
#include "HttpModule.h"
#include "Interfaces/IHttpRequest.h"
#include "Interfaces/IHttpResponse.h"
#include "Misc/FileHelper.h"
#include "Misc/Paths.h"
#include "uba_core.h"

#include <chrono>
#include <future>
#include <memory>
#include <mutex>

using namespace uba;

namespace {

FString ToUE(const std::string& s) { return UTF8_TO_TCHAR(s.c_str()); }
std::string ToCore(const FString& s) { return TCHAR_TO_UTF8(*s); }

TMap<FString, FString> ToUEMap(const std::map<std::string, std::string>& In)
{
    TMap<FString, FString> Out;
    for (const auto& [Key, Value] : In)
    {
        Out.Add(ToUE(Key), ToUE(Value));
    }
    return Out;
}

std::map<std::string, std::string> ToCoreMap(const TMap<FString, FString>& In)
{
    std::map<std::string, std::string> Out;
    for (const auto& [Key, Value] : In)
    {
        Out[ToCore(Key)] = ToCore(Value);
    }
    return Out;
}

/// FHttpModule 桥接：异步 HTTP → 同步 Send（std::promise/future 等待）。
/// 注意：UbaClient::Flush 与其他端一致是同步语义；调用方避免在游戏线程高频 Flush。
class FHttpTransportUE final : public IUbaTransport
{
public:
    UbaHttpResult Send(const std::string& Url, const std::string& Body,
                       int TimeoutMs) override
    {
        FHttpModule& Http = FHttpModule::Get();
        TSharedRef<IHttpRequest, ESPMode::ThreadSafe> Request = Http.CreateHttpRequest();
        Request->SetURL(UTF8_TO_TCHAR(Url.c_str()));
        Request->SetVerb(TEXT("POST"));
        Request->SetHeader(TEXT("Content-Type"), TEXT("application/json"));
        Request->SetContentAsString(UTF8_TO_TCHAR(Body.c_str()));

        std::promise<UbaHttpResult> Promise;
        std::future<UbaHttpResult> Future = Promise.get_future();
        Request->OnProcessRequestComplete() =
            [&Promise](FHttpRequestPtr, const FHttpResponsePtr& Response, bool bSucceeded) {
                if (bSucceeded && Response.IsValid())
                {
                    Promise.set_value({Response.GetResponseCode(),
                                       TCHAR_TO_UTF8(*Response.GetContentAsString())});
                }
                else
                {
                    Promise.set_value({0, "network error"});
                }
            };

        Request->SetTimeout(TimeoutMs / 1000.0f);
        if (!Request->ProcessRequest())
        {
            return {0, "process request failed"};
        }

        // 等待上限略高于单请求超时（服务端 10s）
        if (Future.wait_for(std::chrono::milliseconds(15000)) == std::future_status::ready)
        {
            return Future.get();
        }
        Request->CancelRequest();
        return {0, "wait timeout"};
    }
};

/// 文件持久化设备 ID（Saved/Uba/device_id.txt）——稳定设备标识 + 首装判定。
class FFileStoreUE
{
public:
    static FString DeviceIdPath()
    {
        return FPaths::Combine(FPaths::ProjectSavedDir(), TEXT("Uba"), TEXT("device_id.txt"));
    }

    /// 返回 {deviceId, isNew}：文件不存在即首装（生成并持久化）。
    static TPair<FString, bool> LoadOrCreateDeviceId()
    {
        FString Content;
        if (FFileHelper::LoadFileToString(Content, *DeviceIdPath()) && !Content.IsEmpty())
        {
            return {Content, false};
        }
        FString NewId = FGuid::NewGuid().ToString(EGuidFormats::DigitsWithHyphensLower);
        NewId.ToLowerInline();
        FFileHelper::SaveStringToFile(NewId, *DeviceIdPath());
        return {NewId, true};
    }
};

/// 核心客户端持有器（Pimpl，避免 UE 反射器接触核心头）。
struct FUbaClientHolder
{
    std::shared_ptr<UbaClient> Client;
    std::mutex Mutex;
};

} // namespace

void UUbaSubsystem::Init(const FUbaSettings& Settings)
{
    if (Holder.IsValid())
    {
        return; // 幂等
    }

    const TPair<FString, bool> Device = FFileStoreUE::LoadOrCreateDeviceId();

    UbaConfig Config;
    Config.app_id = ToCore(Settings.AppId);
    Config.app_secret = ToCore(Settings.AppSecret);
    Config.endpoint = ToCore(Settings.Endpoint);
    Config.app_version = ToCore(Settings.AppVersion);
    Config.channel = ToCore(Settings.Channel);
    Config.debug = Settings.Debug;
    Config.device_id = ToCore(Device.Key);
    Config.platform = "unreal";
    Config.page_view_event = "page_view";

    Holder = MakeUnique<FUbaClientHolder>();
    Holder->Client = std::make_shared<UbaClient>(Config, std::make_shared<FHttpTransportUE>());

    // 首装自动埋点（deviceId 本次新建即视为新用户安装，与 Flutter/微信端语义一致）
    if (Config.auto_track_launch && Device.Value)
    {
        Holder->Client->Track("app_launch", {{"first_launch", "true"}}, {});
    }
}

bool UUbaSubsystem::IsInitialized() const
{
    return Holder.IsValid();
}

void UUbaSubsystem::Track(const FString& EventName, const TMap<FString, FString>& Properties,
                          const FString& EventCategory, const FString& ObjectType,
                          const FString& ObjectId, int32 Score, const FString& Amount,
                          const FString& ServerId, int32 Level)
{
    if (!Holder.IsValid())
    {
        return;
    }
    UbaTrackOptions O;
    if (!EventCategory.IsEmpty())
    {
        O.event_category = ToCore(EventCategory);
    }
    if (!ObjectType.IsEmpty())
    {
        O.object_type = ToCore(ObjectType);
    }
    if (!ObjectId.IsEmpty())
    {
        O.object_id = ToCore(ObjectId);
    }
    if (Score != 0)
    {
        O.score = Score;
    }
    if (!Amount.IsEmpty())
    {
        O.amount = ToCore(Amount);
    }
    if (!ServerId.IsEmpty())
    {
        O.server_id = ToCore(ServerId);
    }
    if (Level != 0)
    {
        O.level = Level;
    }
    O.properties = ToCoreMap(Properties);
    Holder->Client->Track(ToCore(EventName), {}, O);
}

void UUbaSubsystem::TrackRisk(const FString& EventName, const FString& RiskType,
                              const FString& RiskLevel, float RiskScore,
                              const FString& Description, const TMap<FString, FString>& Evidence)
{
    if (!Holder.IsValid())
    {
        return;
    }
    UbaRiskEvent Risk;
    if (!RiskType.IsEmpty())
    {
        Risk.risk_type = ToCore(RiskType);
    }
    if (!RiskLevel.IsEmpty())
    {
        Risk.risk_level = ToCore(RiskLevel);
    }
    Risk.risk_score = static_cast<double>(RiskScore);
    if (!Description.IsEmpty())
    {
        Risk.description = ToCore(Description);
    }
    Risk.evidence = ToCoreMap(Evidence);
    Holder->Client->TrackRisk(ToCore(EventName), Risk);
}

void UUbaSubsystem::Identify(int64 UserId)
{
    if (!Holder.IsValid())
    {
        return;
    }
    Holder->Client->Identify(UserId);
}

void UUbaSubsystem::ResetUser()
{
    if (!Holder.IsValid())
    {
        return;
    }
    Holder->Client->ResetUser();
}

void UUbaSubsystem::Flush()
{
    if (!Holder.IsValid())
    {
        return;
    }
    Holder->Client->Flush();
}

void UUbaSubsystem::Shutdown()
{
    if (!Holder.IsValid())
    {
        return;
    }
    Holder->Client->Close();
    Holder.Reset();
}

void UUbaSubsystem::Deinitialize()
{
    Shutdown();
    Super::Deinitialize();
}
