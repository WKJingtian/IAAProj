using System;
using System.Collections.Generic;
using System.Runtime.CompilerServices;
using UnityEngine;

public sealed class MessageBus
{
    private sealed class TypedChannel
    {
        public TypedChannel(Type signature)
        {
            Signature = signature;
            Listeners = new Dictionary<object, Delegate>(ReferenceEqualityComparer.Instance);
        }

        public Type Signature { get; }
        public Dictionary<object, Delegate> Listeners { get; }
    }

    private sealed class DynamicChannel
    {
        public DynamicChannel(Type[] parameterTypes)
        {
            ParameterTypes = parameterTypes;
            Listeners = new Dictionary<object, Action<object[]>>(ReferenceEqualityComparer.Instance);
        }

        public Type[] ParameterTypes { get; }
        public Dictionary<object, Action<object[]>> Listeners { get; }
    }

    private sealed class ReferenceEqualityComparer : IEqualityComparer<object>
    {
        public static readonly ReferenceEqualityComparer Instance = new ReferenceEqualityComparer();

        public new bool Equals(object x, object y)
        {
            return ReferenceEquals(x, y);
        }

        public int GetHashCode(object obj)
        {
            return RuntimeHelpers.GetHashCode(obj);
        }
    }

    private readonly object _syncRoot = new object();
    private readonly Dictionary<string, TypedChannel> _typedChannels = new Dictionary<string, TypedChannel>();
    private readonly Dictionary<string, DynamicChannel> _dynamicChannels = new Dictionary<string, DynamicChannel>();

    public static MessageBus Global { get; } = new MessageBus();

    public void Listen(string message, object owner, Action handler)
    {
        ListenTyped(message, owner, handler, typeof(Action));
    }

    public void Listen<T1>(string message, object owner, Action<T1> handler)
    {
        ListenTyped(message, owner, handler, typeof(Action<T1>));
    }

    public void Listen<T1, T2>(string message, object owner, Action<T1, T2> handler)
    {
        ListenTyped(message, owner, handler, typeof(Action<T1, T2>));
    }

    public void Listen<T1, T2, T3>(string message, object owner, Action<T1, T2, T3> handler)
    {
        ListenTyped(message, owner, handler, typeof(Action<T1, T2, T3>));
    }

    public void Listen<T1, T2, T3, T4>(string message, object owner, Action<T1, T2, T3, T4> handler)
    {
        ListenTyped(message, owner, handler, typeof(Action<T1, T2, T3, T4>));
    }

    public void ListenDynamic(string message, object owner, Action<object[]> handler, params Type[] parameterTypes)
    {
        if (handler == null)
        {
            throw new ArgumentNullException(nameof(handler));
        }

        ValidateMessage(message);
        ValidateOwner(owner);
        Type[] normalizedTypes = NormalizeTypes(parameterTypes);

        lock (_syncRoot)
        {
            if (!_dynamicChannels.TryGetValue(message, out DynamicChannel channel))
            {
                channel = new DynamicChannel(normalizedTypes);
                _dynamicChannels.Add(message, channel);
            }
            else
            {
                EnsureDynamicSignatureMatch(message, channel.ParameterTypes, normalizedTypes);
            }

            EnsureOwnerNotRegistered(message, owner, channel.Listeners.ContainsKey(owner));
            channel.Listeners.Add(owner, handler);
        }
    }

    public void Unlisten(string message, object owner)
    {
        if (string.IsNullOrWhiteSpace(message) || owner == null)
        {
            return;
        }

        lock (_syncRoot)
        {
            if (_typedChannels.TryGetValue(message, out TypedChannel typedChannel))
            {
                typedChannel.Listeners.Remove(owner);
                if (typedChannel.Listeners.Count == 0)
                {
                    _typedChannels.Remove(message);
                }
            }

            if (_dynamicChannels.TryGetValue(message, out DynamicChannel dynamicChannel))
            {
                dynamicChannel.Listeners.Remove(owner);
                if (dynamicChannel.Listeners.Count == 0)
                {
                    _dynamicChannels.Remove(message);
                }
            }
        }
    }

    public void UnlistenAll(object owner)
    {
        if (owner == null)
        {
            return;
        }

        lock (_syncRoot)
        {
            RemoveOwnerFromTypedChannels(owner);
            RemoveOwnerFromDynamicChannels(owner);
        }
    }

    public int Dispatch(string message)
    {
        return DispatchTyped(message, typeof(Action), action => ((Action)action).Invoke());
    }

    public int Dispatch<T1>(string message, T1 arg1)
    {
        return DispatchTyped(message, typeof(Action<T1>), action => ((Action<T1>)action).Invoke(arg1));
    }

    public int Dispatch<T1, T2>(string message, T1 arg1, T2 arg2)
    {
        return DispatchTyped(message, typeof(Action<T1, T2>), action => ((Action<T1, T2>)action).Invoke(arg1, arg2));
    }

    public int Dispatch<T1, T2, T3>(string message, T1 arg1, T2 arg2, T3 arg3)
    {
        return DispatchTyped(message, typeof(Action<T1, T2, T3>), action => ((Action<T1, T2, T3>)action).Invoke(arg1, arg2, arg3));
    }

    public int Dispatch<T1, T2, T3, T4>(string message, T1 arg1, T2 arg2, T3 arg3, T4 arg4)
    {
        return DispatchTyped(message, typeof(Action<T1, T2, T3, T4>), action => ((Action<T1, T2, T3, T4>)action).Invoke(arg1, arg2, arg3, arg4));
    }

    public int DispatchDynamic(string message, params object[] args)
    {
        ValidateMessage(message);

        KeyValuePair<object, Action<object[]>>[] listeners;
        Type[] expectedTypes;
        object[] payload = args ?? Array.Empty<object>();

        lock (_syncRoot)
        {
            if (!_dynamicChannels.TryGetValue(message, out DynamicChannel channel))
            {
                return 0;
            }

            expectedTypes = channel.ParameterTypes;
            ValidateDynamicArguments(message, expectedTypes, payload);
            listeners = Snapshot(channel.Listeners);
        }

        int invokedCount = 0;
        List<object> unavailableOwners = null;

        for (int i = 0; i < listeners.Length; i++)
        {
            object owner = listeners[i].Key;
            if (!IsOwnerAvailable(owner))
            {
                ReportUnavailableOwner(message, owner);
                if (unavailableOwners == null)
                {
                    unavailableOwners = new List<object>();
                }

                unavailableOwners.Add(owner);
                continue;
            }

            object[] copy = new object[payload.Length];
            Array.Copy(payload, copy, payload.Length);
            listeners[i].Value.Invoke(copy);
            invokedCount++;
        }

        if (unavailableOwners != null)
        {
            RemoveDynamicListeners(message, unavailableOwners);
        }

        return invokedCount;
    }

    private void ListenTyped(string message, object owner, Delegate handler, Type signature)
    {
        if (handler == null)
        {
            throw new ArgumentNullException(nameof(handler));
        }

        ValidateMessage(message);
        ValidateOwner(owner);

        lock (_syncRoot)
        {
            if (!_typedChannels.TryGetValue(message, out TypedChannel channel))
            {
                channel = new TypedChannel(signature);
                _typedChannels.Add(message, channel);
            }
            else if (channel.Signature != signature)
            {
                throw new InvalidOperationException(
                    $"Message '{message}' is already registered as '{channel.Signature.Name}', cannot listen with '{signature.Name}'.");
            }

            EnsureOwnerNotRegistered(message, owner, channel.Listeners.ContainsKey(owner));
            channel.Listeners.Add(owner, handler);
        }
    }

    private int DispatchTyped(string message, Type signature, Action<Delegate> invoker)
    {
        ValidateMessage(message);

        KeyValuePair<object, Delegate>[] listeners;

        lock (_syncRoot)
        {
            if (!_typedChannels.TryGetValue(message, out TypedChannel channel))
            {
                return 0;
            }

            if (channel.Signature != signature)
            {
                throw new InvalidOperationException(
                    $"Message '{message}' was registered as '{channel.Signature.Name}', cannot dispatch as '{signature.Name}'.");
            }

            listeners = Snapshot(channel.Listeners);
        }

        int invokedCount = 0;
        List<object> unavailableOwners = null;

        for (int i = 0; i < listeners.Length; i++)
        {
            object owner = listeners[i].Key;
            if (!IsOwnerAvailable(owner))
            {
                ReportUnavailableOwner(message, owner);
                if (unavailableOwners == null)
                {
                    unavailableOwners = new List<object>();
                }

                unavailableOwners.Add(owner);
                continue;
            }

            invoker(listeners[i].Value);
            invokedCount++;
        }

        if (unavailableOwners != null)
        {
            RemoveTypedListeners(message, unavailableOwners);
        }

        return invokedCount;
    }

    private void RemoveTypedListeners(string message, List<object> owners)
    {
        lock (_syncRoot)
        {
            if (!_typedChannels.TryGetValue(message, out TypedChannel channel))
            {
                return;
            }

            for (int i = 0; i < owners.Count; i++)
            {
                channel.Listeners.Remove(owners[i]);
            }

            if (channel.Listeners.Count == 0)
            {
                _typedChannels.Remove(message);
            }
        }
    }

    private void RemoveDynamicListeners(string message, List<object> owners)
    {
        lock (_syncRoot)
        {
            if (!_dynamicChannels.TryGetValue(message, out DynamicChannel channel))
            {
                return;
            }

            for (int i = 0; i < owners.Count; i++)
            {
                channel.Listeners.Remove(owners[i]);
            }

            if (channel.Listeners.Count == 0)
            {
                _dynamicChannels.Remove(message);
            }
        }
    }

    private void RemoveOwnerFromTypedChannels(object owner)
    {
        List<string> emptyChannels = null;

        foreach (KeyValuePair<string, TypedChannel> pair in _typedChannels)
        {
            pair.Value.Listeners.Remove(owner);
            if (pair.Value.Listeners.Count == 0)
            {
                if (emptyChannels == null)
                {
                    emptyChannels = new List<string>();
                }

                emptyChannels.Add(pair.Key);
            }
        }

        if (emptyChannels != null)
        {
            for (int i = 0; i < emptyChannels.Count; i++)
            {
                _typedChannels.Remove(emptyChannels[i]);
            }
        }
    }

    private void RemoveOwnerFromDynamicChannels(object owner)
    {
        List<string> emptyChannels = null;

        foreach (KeyValuePair<string, DynamicChannel> pair in _dynamicChannels)
        {
            pair.Value.Listeners.Remove(owner);
            if (pair.Value.Listeners.Count == 0)
            {
                if (emptyChannels == null)
                {
                    emptyChannels = new List<string>();
                }

                emptyChannels.Add(pair.Key);
            }
        }

        if (emptyChannels != null)
        {
            for (int i = 0; i < emptyChannels.Count; i++)
            {
                _dynamicChannels.Remove(emptyChannels[i]);
            }
        }
    }

    private static KeyValuePair<object, Delegate>[] Snapshot(Dictionary<object, Delegate> listeners)
    {
        KeyValuePair<object, Delegate>[] snapshot = new KeyValuePair<object, Delegate>[listeners.Count];
        int index = 0;
        foreach (KeyValuePair<object, Delegate> pair in listeners)
        {
            snapshot[index++] = pair;
        }

        return snapshot;
    }

    private static KeyValuePair<object, Action<object[]>>[] Snapshot(Dictionary<object, Action<object[]>> listeners)
    {
        KeyValuePair<object, Action<object[]>>[] snapshot = new KeyValuePair<object, Action<object[]>>[listeners.Count];
        int index = 0;
        foreach (KeyValuePair<object, Action<object[]>> pair in listeners)
        {
            snapshot[index++] = pair;
        }

        return snapshot;
    }

    private static bool IsOwnerAvailable(object owner)
    {
        if (owner == null)
        {
            return false;
        }

        if (owner is UnityEngine.Object unityObject)
        {
            return unityObject != null;
        }

        return true;
    }

    private static void ValidateMessage(string message)
    {
        if (string.IsNullOrWhiteSpace(message))
        {
            throw new ArgumentException("Message name cannot be null or whitespace.", nameof(message));
        }
    }

    private static void ValidateOwner(object owner)
    {
        if (owner == null)
        {
            throw new ArgumentNullException(nameof(owner), "Listener owner cannot be null.");
        }

        if (owner.GetType().IsValueType)
        {
            throw new ArgumentException("Listener owner must be a reference type.", nameof(owner));
        }

        if (!IsOwnerAvailable(owner))
        {
            throw new InvalidOperationException("Listener owner is not available.");
        }
    }

    private static void EnsureOwnerNotRegistered(string message, object owner, bool isRegistered)
    {
        if (isRegistered)
        {
            throw new InvalidOperationException(
                $"Owner '{owner.GetType().Name}' has already listened to message '{message}'. Unlisten first before re-listening.");
        }
    }

    private static Type[] NormalizeTypes(Type[] parameterTypes)
    {
        if (parameterTypes == null || parameterTypes.Length == 0)
        {
            return Array.Empty<Type>();
        }

        Type[] copy = new Type[parameterTypes.Length];
        for (int i = 0; i < parameterTypes.Length; i++)
        {
            Type type = parameterTypes[i];
            if (type == null)
            {
                throw new ArgumentException("Parameter type cannot be null.", nameof(parameterTypes));
            }

            copy[i] = type;
        }

        return copy;
    }

    private static void EnsureDynamicSignatureMatch(string message, Type[] existing, Type[] incoming)
    {
        if (existing.Length != incoming.Length)
        {
            throw new InvalidOperationException(
                $"Message '{message}' expects {existing.Length} argument(s), but received signature with {incoming.Length} argument(s).");
        }

        for (int i = 0; i < existing.Length; i++)
        {
            if (existing[i] != incoming[i])
            {
                throw new InvalidOperationException(
                    $"Message '{message}' argument #{i + 1} expects '{existing[i].Name}', but got '{incoming[i].Name}'.");
            }
        }
    }

    private static void ValidateDynamicArguments(string message, Type[] expectedTypes, object[] payload)
    {
        if (expectedTypes.Length != payload.Length)
        {
            throw new InvalidOperationException(
                $"Message '{message}' expects {expectedTypes.Length} argument(s), but got {payload.Length}.");
        }

        for (int i = 0; i < expectedTypes.Length; i++)
        {
            Type expectedType = expectedTypes[i];
            object value = payload[i];

            if (!IsValueCompatible(expectedType, value))
            {
                string actualName = value == null ? "null" : value.GetType().Name;
                throw new InvalidOperationException(
                    $"Message '{message}' argument #{i + 1} expects '{expectedType.Name}', but got '{actualName}'.");
            }
        }
    }

    private static bool IsValueCompatible(Type expectedType, object value)
    {
        if (value == null)
        {
            return !expectedType.IsValueType || Nullable.GetUnderlyingType(expectedType) != null;
        }

        return expectedType.IsInstanceOfType(value);
    }

    private static void ReportUnavailableOwner(string message, object owner)
    {
        string ownerName = owner == null ? "null" : owner.GetType().Name;
        Debug.LogError($"[MessageBus] Message '{message}' skipped an unavailable owner '{ownerName}'. Listener has been removed.");
    }
}
