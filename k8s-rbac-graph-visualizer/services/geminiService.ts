import { GoogleGenAI } from "@google/genai";
import { RbacNode } from "../types";

const apiKey = process.env.API_KEY || '';

export const analyzeRbacNode = async (node: RbacNode): Promise<string> => {
  if (!apiKey) {
    return "API Key not configured. Please set process.env.API_KEY.";
  }

  try {
    const ai = new GoogleGenAI({ apiKey });
    
    let prompt = "";
    
    if (node.type === 'Role' || node.type === 'ClusterRole') {
      prompt = `
        Analyze the following Kubernetes ${node.type} for security risks.
        Node Name: ${node.name}
        Rules: ${JSON.stringify(node.rules, null, 2)}
        
        Please provide:
        1. A summary of what this role allows.
        2. Potential security risks (e.g., privilege escalation, secrets access).
        3. A rating of risk (Low, Medium, High, Critical).
        Keep it concise.
      `;
    } else if (node.type === 'User' || node.type === 'ServiceAccount') {
        prompt = `
        Explain the identity of this Kubernetes Subject.
        Type: ${node.type}
        ID: ${node.id}
        
        Explain generally what kind of actor this represents in a K8s cluster.
        `;
    } else {
       prompt = `Explain the function of this Kubernetes object:
       Type: ${node.type}
       Name: ${node.name}
       Data: ${JSON.stringify(node, null, 2)}
       `;
    }

    const response = await ai.models.generateContent({
      model: 'gemini-2.5-flash',
      contents: prompt,
    });

    return response.text || "No analysis generated.";
  } catch (error) {
    console.error("Gemini API Error:", error);
    return "Failed to generate analysis. Please check console for details.";
  }
};
