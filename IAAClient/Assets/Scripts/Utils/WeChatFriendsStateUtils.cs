using System;
using System.Runtime.InteropServices;
using System.Threading.Tasks;

public class WeChatFriendsStateUtils : MonoSingleton<WeChatFriendsStateUtils>
{
    private TaskCompletionSource<string> _pendingRequest;

    [DllImport("__Internal")]
    private static extern void IAA_RequestFriendsStateDataWithCallback(string gameObjectName);

    public Task<string> RequestAsync()
    {
        if (_pendingRequest != null && !_pendingRequest.Task.IsCompleted)
        {
            return _pendingRequest.Task;
        }

        _pendingRequest = new TaskCompletionSource<string>();
        IAA_RequestFriendsStateDataWithCallback(gameObject.name);
        return _pendingRequest.Task;
    }

    public void OnFriendsStateDataSuccess(string payload)
    {
        _pendingRequest?.TrySetResult(payload);
        _pendingRequest = null;
    }

    public void OnFriendsStateDataFailed(string error)
    {
        _pendingRequest?.TrySetException(new Exception(error));
        _pendingRequest = null;
    }
}
